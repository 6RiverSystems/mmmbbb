package services

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	grpccommon "go.6river.tech/gosix/grpc"
	"go.6river.tech/gosix/logging"
	"go.6river.tech/gosix/pubsub"
	"go.6river.tech/gosix/registry"
	"go.6river.tech/gosix/server"
	"go.6river.tech/gosix/testutils"
	"go.6river.tech/mmmbbb/defaults"
	"go.6river.tech/mmmbbb/ent"
	"go.6river.tech/mmmbbb/ent/enttest"
)

func safeName(t testing.TB) string {
	name := t.Name()
	name = strings.ReplaceAll(name, "/", "_")
	return name
}

func initGrpcTest(t testing.TB) (client *ent.Client, psClient pubsub.Client) {
	return initGrpcService(t, nil), initPubsubClient(t)
}

func initGrpcService(t testing.TB, reg *registry.Registry) *ent.Client {
	logging.ConfigureDefaultLogging()
	server.EnableRandomPorts()
	client := enttest.ClientForTest(t)
	ctx := testutils.ContextForTest(t)
	svc := grpccommon.NewGrpcService(
		defaults.Port, defaults.GRPCOffset,
		nil,
		InitializeGrpcServers,
	)
	require.NoError(t, svc.Initialize(ctx, reg, client))
	t.Cleanup(func() {
		// TODO: using the test context may not be great here, if it's already
		// canceled. on the other hand, we want to bound cleanup.
		assert.NoError(t, svc.Cleanup(ctx, reg))
	})

	eg, egCtx := errgroup.WithContext(ctx)

	ready := make(chan struct{})
	eg.Go(func() error {
		return svc.Start(egCtx, ready)
	})

	select {
	case <-egCtx.Done():
		// timeout or error
		err := eg.Wait()
		if err == nil {
			err = egCtx.Err()
		}
		// this is definitely going to fail
		require.NoError(t, err, "gRPC service init must succeed")
	case <-ready:
		// grpc server is ready, continue on
	}
	return client
}

func initPubsubClient(t testing.TB) pubsub.Client {
	realPort := server.ResolvePort(defaults.Port, defaults.GRPCOffset)
	oldEmuEnv, oldHadEmuEnv := os.LookupEnv("PUBSUB_EMULATOR_HOST")
	t.Cleanup(func() {
		if oldHadEmuEnv {
			os.Setenv("PUBSUB_EMULATOR_HOST", oldEmuEnv)
		} else {
			os.Unsetenv("PUBSUB_EMULATOR_HOST")
		}
	})
	os.Setenv("PUBSUB_EMULATOR_HOST", "localhost:"+strconv.Itoa(realPort))

	// use a random namespace each time to avoid prom errors when running go test
	// -count N
	var err error
	psClient, err := pubsub.NewClient(
		testutils.ContextForTest(t),
		safeName(t),
		"__test__"+strings.ReplaceAll(uuid.NewString(), "-", ""),
		nil,
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, psClient.Close()) })
	return psClient
}
