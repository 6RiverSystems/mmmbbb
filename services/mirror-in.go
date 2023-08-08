// Copyright (c) 2021 6 River Systems
//
// Permission is hereby granted, free of charge, to any person obtaining a copy of
// this software and associated documentation files (the "Software"), to deal in
// the Software without restriction, including without limitation the rights to
// use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
// the Software, and to permit persons to whom the Software is furnished to do so,
// subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
// FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
// COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
// IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
// CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.6river.tech/gosix/logging"
	"go.6river.tech/gosix/pubsub"
	"go.6river.tech/gosix/registry"
	"go.6river.tech/mmmbbb/actions"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/topic"
)

// topicMirrorIn manages mirroring google pubsub topics to mmmbbb
type topicMirrorIn struct {
	// config

	project           string
	remoteToLocalName func(string) string
	localToRemoteName func(string) string
	topicFilter       func(string) bool

	// state

	logger      *logging.Logger
	hostName    string
	client      *ent.Client
	psClient    pubsub.Client
	subWatchers map[string]monitoredGroup
}

func (s *topicMirrorIn) Name() string {
	return fmt.Sprintf("topic-mirror-in(%s)", s.project)
}

func (s *topicMirrorIn) Initialize(ctx context.Context, _ *registry.Registry, client *ent.Client) error {
	if s.project == "" {
		return errors.New("project config missing")
	}
	if s.remoteToLocalName == nil {
		return errors.New("remoteToLocalName config missing")
	}
	if s.localToRemoteName == nil {
		return errors.New("localToRemoteName config missing")
	}
	if s.topicFilter == nil {
		// no filter = all topics
		s.topicFilter = func(string) bool { return true }
	}

	s.client = client
	s.subWatchers = map[string]monitoredGroup{}
	s.logger = logging.GetLogger("services/topic-mirror-in/" + s.project)

	var err error

	// in k8s, hostname will be the pod name
	if s.hostName, err = os.Hostname(); err != nil {
		return err
	}

	// context here is just for running initialization, whereas what we pass to
	// NewClient will hang around, so we give that the background context and will
	// handle stopping it differently
	s.psClient, err = pubsub.NewClient(context.Background(), s.project, "topic_mirror_in", nil)
	if err != nil {
		return err
	}
	return nil
}

func (s *topicMirrorIn) Start(ctx context.Context, ready chan<- struct{}) error {
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

func (s *topicMirrorIn) monitorTopics(ctx context.Context, ready chan<- struct{}) error {
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

func (s *topicMirrorIn) startMirrorsOnce(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	subs := map[string]struct{ name string }{}

	// TODO: also monitor orphaned mirror subs until they are empty: need to
	// check if the sub's topic is still valid

	topicIterator := s.psClient.Topics(ctx)
	for {
		t, err := topicIterator.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		// topic.ID() is the "short" name, as opposed to topic.String() which is
		// the "fully qualified" name
		if !s.topicFilter(t.ID()) {
			continue
		}

		// create the local topic and remote subscription. doing this in this
		// loop, instead of just in the watchSub startup helps ensure two-way
		// replication recovers reasonably quickly after deletion on one side

		tc, err := t.Config(ctx)
		if err != nil {
			return err
		}

		// TODO: poll for changes (e.g. to labels) and apply them to the local topic

		localTopicName := s.remoteToLocalName(t.ID())
		ct := actions.NewCreateTopic(actions.CreateTopicParams{
			Name:   localTopicName,
			Labels: tc.Labels,
		})
		err = s.client.DoTx(ctx, nil, func(tx *ent.Tx) error {
			err := ct.Execute(ctx, tx)
			if err != nil {
				if !errors.Is(err, actions.ErrExists) {
					return err
				}
				_, err := tx.Topic.Query().
					Where(
						topic.Name(localTopicName),
						topic.DeletedAtIsNil(),
					).
					Only(ctx)
				if err != nil {
					return err
				}
			} else {
				results, _ := ct.Results()
				s.logger.Info().
					Str("topicName", localTopicName).
					Stringer("topicID", results.ID).
					Msg("Created local mirror topic")
			}
			return nil
		})
		if err != nil {
			return err
		}
		createdTopicsCounter.WithLabelValues(directionInbound).Inc()

		sn := s.subName(t.ID())
		subs[sn] = struct{ name string }{t.ID()}

		sub, err := t.CreateSubscription(ctx, sn, pubsub.SubscriptionConfig{
			// we need to enable message ordering on the google side in order to
			// allow it to be used on the local side
			EnableMessageOrdering: true,
			// short TTL on this sub and its messages
			RetentionDuration: 24 * time.Hour,
			ExpirationPolicy:  24 * time.Hour,
		})
		if err != nil {
			if status.Code(err) != codes.AlreadyExists {
				return err
			}
			if sub == nil {
				sub = s.psClient.Subscription(sn)
			}
		} else {
			s.logger.Info().
				Str("topicName", localTopicName).
				Str("psTopicName", t.ID()).
				Str("psSubName", sn).
				Msg("Created remote mirror subscription")
		}
		// NOTE: we can't "fix" EnableMessageOrdering if it's wrong
		if _, err = sub.EnsureDefaultConfig(ctx); err != nil {
			return err
		}

		// TODO: check sub is not attached to a deleted sub, delete it and try
		// again if so
	}

	// harvest any dead watchers
	for sn, subMon := range s.subWatchers {
		select {
		case <-subMon.Done():
			watchersEndedCounter.WithLabelValues(directionInbound).Inc()
			if err := subMon.Wait(); err != nil {
				// TODO: we want to log the topic name too
				s.logger.Error().Err(err).Str("psSubName", sn).Msg("Mirror subscriber died")
			} else {
				// TODO: we don't expect mirrors to end without error, and if they did
				// we wouldn't know as the context wouldn't be canceled
				s.logger.Debug().Str("psSubName", sn).Msg("Mirror subscriber ended")
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
			return s.watchSub(subCtx, sn, ti.name)
		})
		watchersStartedCounter.WithLabelValues(directionInbound).Inc()
	}

	mirroredTopicsGauge.WithLabelValues(directionInbound).Set(float64(len(s.subWatchers)))

	return nil
}

func (s *topicMirrorIn) watchSub(ctx context.Context, psSubName, psTopicName string) error {
	localTopicName := s.remoteToLocalName(psTopicName)

	logger := s.logger.With(func(c zerolog.Context) zerolog.Context {
		return c.
			Str("psSubName", psSubName).
			Str("psTopicName", psTopicName).
			// Stringer("topicID", localTopicID).
			Str("topicName", localTopicName)
	})
	logger.Debug().Msg("Starting message mirror")

	sub := s.psClient.Subscription(psSubName)
	// TODO: can we disable synchronous receive here?
	sub.ReceiveSettings().Synchronous = true

	// TODO: poll sub in case its topic gets deleted, abort if it does

	// google pubsub will stop receive when the ctx completes
	err := sub.Receive(ctx, func(ctx context.Context, m pubsub.Message) {
		// loop avoidance: don't copy any messages that came from topicMirrorOut
		for _, a := range psMirrorAttrs {
			if _, ok := m.RealMessage().Attributes[a]; ok {
				messagesDroppedCounter.WithLabelValues(directionInbound, dropReasonLoop).Inc()
				m.Ack()
				return
			}
		}

		// TODO: batch messages into local transactions for efficiency

		// decode to a rawmessage just to validate json
		var mp json.RawMessage
		if err := json.Unmarshal(m.RealMessage().Data, &mp); err != nil {
			messagesDroppedCounter.WithLabelValues(directionInbound, dropReasonNotJSON).Inc()
			logger.Error().Err(err).Str("messageID", m.RealMessage().ID).Msg("Got non-JSON message, dropping it")
			m.Ack()
			return
		}

		p := actions.NewPublishMessage(actions.PublishMessageParams{
			TopicName:  localTopicName,
			OrderKey:   m.RealMessage().OrderingKey,
			Payload:    mp,
			Attributes: m.RealMessage().Attributes,
		})
		if err := s.client.DoCtxTx(ctx, nil, p.Execute); err != nil {
			logger.Error().Err(err).Msg("Failed to publish, NACKing for retry")
			m.Nack()
			return
		}
		messagesMirroredCounter.WithLabelValues(directionInbound).Inc()

		m.Ack()
	})
	if err != nil {
		return err
	}

	return nil
}

var mirrorInSubNs = uuid.MustParse("3bb60d85-3760-40dd-90fb-68d6c4a4c9f8")

func (s *topicMirrorIn) subName(psTopicName string) string {
	localTopicName := s.remoteToLocalName(psTopicName)
	hashBytes := make([]byte, 0, 2+len(psTopicName)+len(localTopicName)+len(s.project))
	hashBytes = append(hashBytes, psTopicName...)
	hashBytes = append(hashBytes, 0)
	hashBytes = append(hashBytes, localTopicName...)
	hashBytes = append(hashBytes, 0)
	hashBytes = append(hashBytes, s.project...)
	subHash := uuid.NewSHA1(mirrorInSubNs, hashBytes)
	// we stripped off the site name prefix before, we need to add it back here
	subName := s.localToRemoteName(fmt.Sprintf("mirror-%s-%s", subHash, localTopicName))
	// obey name length limits for google pubsub. we put the UUID first so we'll
	// still have a unique value even if we truncate, replace end with ellipsis to
	// make truncation clear
	if len(subName) > 255 {
		subName = subName[0:252] + "..."
	}
	return subName
}

func (s *topicMirrorIn) Cleanup(ctx context.Context, _ *registry.Registry) error {
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
	if site := os.Getenv("MIRROR_IN_SITE_NAME"); site != "" {
		remoteToLocalName := func(name string) string {
			// reverse of what outbound mirror does
			// strip the site name
			if strings.HasPrefix(name, site+"-") {
				return name[len(site)+1:]
			}
			// inapplicable, return a magic value
			return ""
		}
		// this is just a PoC placeholder
		var regexFilter func(string) bool
		if tfRegex := os.Getenv("MIRROR_TOPIC_REGEX"); tfRegex != "" {
			tfMatcher := regexp.MustCompile(tfRegex)
			regexFilter = func(topicName string) bool {
				mapped := remoteToLocalName(topicName)
				return mapped != "" && tfMatcher.MatchString(mapped)
			}
		}
		shardFilter := parseShardConfig()
		defaultServices = append(defaultServices, &topicMirrorIn{
			project:           pubsub.DefaultProjectId(),
			remoteToLocalName: remoteToLocalName,
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
