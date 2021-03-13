package services

import (
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"go.6river.tech/gosix/logging"
	"go.6river.tech/gosix/pubsub"
	"go.6river.tech/gosix/registry"
	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/topic"
)

// TODO: metrics

// topicMirrorOut manages mirroring mmmbbb topics to google pubsub
type topicMirrorOut struct {
	// config

	project           string
	localToRemoteName func(string) string
	topicFilter       func(string) bool

	// state

	logger      *logging.Logger
	hostName    string
	client      *ent.Client
	psClient    pubsub.Client
	subWatchers map[string]monitoredGroup
}

func (s *topicMirrorOut) Name() string {
	return fmt.Sprintf("topic-mirror-out(%s)", s.project)
}

func (s *topicMirrorOut) Initialize(ctx context.Context, _ *registry.Registry, client *ent.Client) error {
	if s.project == "" {
		return errors.New("project config missing")
	}
	if s.localToRemoteName == nil {
		return errors.New("nameMapper config missing")
	}
	if s.topicFilter == nil {
		// no filter = all topics
		s.topicFilter = func(string) bool { return true }
	}

	s.client = client
	s.subWatchers = map[string]monitoredGroup{}
	s.logger = logging.GetLogger("services/topic-mirror-out/" + s.project)

	var err error

	// in k8s, hostname will be the pod name
	if s.hostName, err = os.Hostname(); err != nil {
		return err
	}

	// context here is just for running initialization, whereas what we pass to
	// NewClient will hang around, so we give that the background context and will
	// handle stopping it differently
	s.psClient, err = pubsub.NewClient(context.Background(), s.project, "topic_mirror_out", nil)
	if err != nil {
		return err
	}
	return nil
}

func (s *topicMirrorOut) Start(ctx context.Context, ready chan<- struct{}) error {
	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error { return s.monitorTopics(egCtx, ready) })

	err := eg.Wait()

	// wait for all the sub watchers to finish before returning
	for _, subCtx := range s.subWatchers {
		<-subCtx.Done()
		// take the first error if we don't have one already
		if err == nil {
			err = subCtx.Err()
		}
	}

	return err
}

func (s *topicMirrorOut) monitorTopics(ctx context.Context, ready chan<- struct{}) error {
	var topicAwaiter actions.AnyTopicModifiedNotifier
	defer func() { actions.CancelAnyTopicModifiedAwaiter(topicAwaiter) }()
	for {
		// get a fresh notifier every loop, before we query, so that there isn't a
		// race from a change between query and notifier setup
		actions.CancelAnyTopicModifiedAwaiter(topicAwaiter)
		topicAwaiter = actions.AnyTopicModifiedAwaiter()
		if err := s.startMirrorsOnce(ctx); err != nil {
			if ready != nil {
				close(ready)
			}
			return err
		}
		if ready != nil {
			close(ready)
			ready = nil
		}
		if r := waitMonitors(ctx, time.Minute, topicAwaiter, s.subWatchers); r == waitMonitorContextDone {
			return ctx.Err()
		}
	}
}

func (s *topicMirrorOut) startMirrorsOnce(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	subs := map[string]struct {
		name string
		id   uuid.UUID
	}{}

	// TODO: also monitor orphaned mirror subs until they are empty

	err := s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
		topics, err := tx.Topic.Query().
			Where(topic.DeletedAtIsNil()).
			All(ctx)
		if err != nil {
			return err
		}

		for _, t := range topics {
			if !s.topicFilter(t.Name) {
				continue
			}
			// create the remote topic and local subscription. doing this in this
			// loop, instead of just in the watchSub startup helps ensure two-way
			// replication recovers reasonably quickly after deletion on one side

			_, err := s.psClient.CreateTopic(ctx, s.localToRemoteName(t.Name))
			if err != nil {
				if status.Code(err) != codes.AlreadyExists {
					return err
				}
			} else {
				s.logger.Info().Str("psTopicName", t.Name).Msg("Created remote mirror topic")
			}

			sn := s.subName(t.Name, t.ID)
			subs[sn] = struct {
				name string
				id   uuid.UUID
			}{t.Name, t.ID}
			cs := actions.NewCreateSubscription(actions.CreateSubscriptionParams{
				TopicName:  t.Name,
				Name:       sn,
				TTL:        24 * time.Hour, // TODO: should this be short or long?
				MessageTTL: 24 * time.Hour, // TODO: should this be short or long?
				// if our sub isn't ordered, then google subs can't be
				OrderedDelivery: true,
			})
			if err := cs.Execute(ctx, tx); err != nil {
				if !errors.Is(err, actions.ErrExists) {
					return err
				}
			} else {
				s.logger.Info().
					Str("topicName", t.Name).
					Stringer("topicID", t.ID).
					Str("subName", sn).
					Msg("Created local mirror subscription")
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// harvest any dead watchers
	for sn, subMon := range s.subWatchers {
		select {
		case <-subMon.Done():
			if err := subMon.Wait(); err != nil {
				// TODO: we want to log the topic name too
				s.logger.Error().Err(err).Str("subName", sn).Msg("Mirror subscriber died")
			} else {
				// TODO: we don't expect mirrors to end without error, and if they did
				// we wouldn't know as the context wouldn't be canceled
				s.logger.Debug().Str("subName", sn).Msg("Mirror subscriber ended")
			}
			delete(s.subWatchers, sn)
		default:
		}
	}

	// make sure we have listeners running for all the subs
	for sn, ti := range subs {
		if _, ok := s.subWatchers[sn]; ok {
			// already have one
			continue
		}
		s.subWatchers[sn] = monitor(ctx, func(subCtx context.Context) error {
			return s.watchSub(subCtx, sn, ti.name, ti.id)
		})
	}
	return nil
}

func (s *topicMirrorOut) watchSub(ctx context.Context, subName, topicName string, topicID uuid.UUID) error {
	psTopicName := s.localToRemoteName(topicName)
	// assumes monitor loop will have handled topic creation
	psTopic := s.psClient.Topic(psTopicName)

	logger := s.logger.With(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("subName", subName).
			Str("topicName", topicName).
			Stringer("topicID", topicID).
			Str("psTopicName", psTopicName)
	})
	logger.Debug().Msg("Starting message mirror")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// TODO: smarter flow control in the presence of many subs
		p := actions.GetSubscriptionMessagesParams{
			Name:        subName,
			MaxMessages: 100,
			MaxBytes:    10 * 1024 * 1024,
		}
		getter := actions.NewGetSubscriptionMessages(p)
		if err := getter.ExecuteClient(ctx, s.client); err != nil {
			return err
		}
		deliveries := getter.Deliveries()
		// if we get no deliveries, and have been orphaned, give up
		if len(deliveries) == 0 {
			if t, err := s.client.Topic.Get(ctx, topicID); err != nil {
				return err
			} else if t.DeletedAt != nil {
				// TODO: check if there are pending deliveries that are blocked on
				// ordering constraints
				logger.Debug().Msg("Orphaned, giving up")
				// allow the auto-expiration to clean up for us, gives a window to
				// recover lost messages
				return nil
			}
		}
		pubResults := make([]*pubsub.PublishResult, len(deliveries))
		for i, d := range deliveries {
			var orderKey string
			if d.OrderKey != nil {
				orderKey = *d.OrderKey
			}
			// queue them all up in parallel
			outAttrs := map[string]string{
				// annotate so we can identify these elsewhere if needed
				psAttrMirroredFrom:    topicName,
				psAttrMirroredVia:     subName,
				psAttrMirroredThrough: s.hostName,
			}
			for k, v := range d.Attributes {
				// don't allow overwrite of our preloaded attrs
				if _, ok := outAttrs[k]; !ok {
					outAttrs[k] = v
				}
			}
			pubResults[i] = psTopic.Publish(ctx, &pubsub.RealMessage{
				OrderingKey: orderKey,
				Data:        d.Payload,
				Attributes:  outAttrs,
			})
		}
		// await all the deliveries, ack them if successful. we could ack just those
		// that succeed instead of an all-or-nothing, but not worth the increased
		// complexity
		acks := make([]uuid.UUID, len(pubResults))
		for i, r := range pubResults {
			if _, err := r.Get(ctx); err != nil {
				return err
			}
			acks[i] = deliveries[i].ID
		}
		ack := actions.NewAckDeliveries(acks...)
		if err := s.client.DoCtxTx(ctx, nil, ack.Execute); err != nil {
			return err
		}
	}
}

const psAttrMirroredFrom = "mirroredFrom"
const psAttrMirroredVia = "mirroredVia"
const psAttrMirroredThrough = "mirroredThrough"

// psMirrorAttrs is the list of out of band attributes attached to google pubsub
// messages sent from topicMirrorOut, to help identifying such messages if they
// are later received elsewhere.
var psMirrorAttrs = []string{psAttrMirroredFrom, psAttrMirroredVia, psAttrMirroredThrough}

var mirrorOutSubNs = uuid.MustParse("91841237-97f2-47e7-95e4-caa990260032")

func (s *topicMirrorOut) subName(topicName string, topicID uuid.UUID) string {
	mappedName := s.localToRemoteName(topicName)
	hashBytes := make([]byte, 0, 3+16+len(topicName)+len(mappedName)+len(s.project))
	hashBytes = append(hashBytes, topicID[:]...)
	hashBytes = append(hashBytes, 0)
	hashBytes = append(hashBytes, topicName...)
	hashBytes = append(hashBytes, 0)
	hashBytes = append(hashBytes, mappedName...)
	hashBytes = append(hashBytes, 0)
	hashBytes = append(hashBytes, s.project...)
	subHash := uuid.NewSHA1(mirrorOutSubNs, hashBytes)
	subName := fmt.Sprintf("mirror-%s-%s", subHash, topicName)
	// obey name length limits for google pubsub. we put the UUID first so we'll
	// still have a unique value even if we truncate, replace end with ellipsis to
	// make truncation clear
	if len(subName) > 255 {
		subName = subName[0:252] + "..."
	}
	return subName
}

func (s *topicMirrorOut) Cleanup(ctx context.Context, _ *registry.Registry) error {
	if s.psClient != nil {
		if err := s.psClient.Close(); err != nil {
			return err
		}
		s.psClient = nil
	}
	s.client = nil
	return nil
}

func init() {
	if site := os.Getenv("MIRROR_OUT_SITE_NAME"); site != "" {
		// this is just a PoC placeholder
		var regexFilter func(string) bool
		if tfRegex := os.Getenv("MIRROR_TOPIC_REGEX"); tfRegex != "" {
			tfMatcher := regexp.MustCompile(tfRegex)
			regexFilter = func(topicName string) bool {
				return tfMatcher.MatchString(topicName)
			}
		}
		shardFilter := parseShardConfig()
		defaultServices = append(defaultServices, &topicMirrorOut{
			project: pubsub.DefaultProjectId(),
			localToRemoteName: func(name string) string {
				return fmt.Sprintf("%s-%s", site, name)
			},
			topicFilter: func(topicName string) bool {
				if regexFilter != nil && !regexFilter(topicName) {
					return false
				}
				if shardFilter != nil && !shardFilter(topicName) {
					return false
				}
				return true
			},
		})
	}
}
