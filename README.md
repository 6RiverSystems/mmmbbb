# mmmbbb

A Message Bus for 6RiverSystems.

This is, in fact, a near drop-in replacement for the Google PubSub emulator.
Specifically it implements most of the PubSub gRPC API, and specifically a
superset of what the emulator does, sufficient for the needs of 6RS.

## Running it

### For Rivs

Given a special branch of `infrastructure`, this will be run automatically by
`6mon`.

### For others

TODO

## Configuration

### Environment variables

TODO

### Supported persistence backends

Both PostgreSQL and SQLite are supported backends. With SQLite, only file-based
storage is currently supported, not memory-based storage, due to issues with
concurrency, locking, and some implementation issues with the Go SQLite driver.

To select a backend: TODO

## What is and isn't implemented

The bulk of the gRPC API is implemented. If your app works with the Google
emulator, it should work with this one too.

Things that are not implemented:

- Region settings
- KMS settings
- The Schema API and any PubSub settings related to it
- Authentication settings for Push subscriptions
- Changing message retry backoff settings for a subscription may not fully take
  effect when actively streaming messages from it (including from an HTTP Push
  configuration)

Things that are different:

- Some default timeouts & expiration delays may have different defaults from the
  Google ones, though they should all still be suitable for development purposes
- You cannot actually turn off the exponential backoff for delivery retries,
  though you can change its settings. If configure a subscription without this
  enabled, a default set of backoff settings will be used and it will still be
  active under the hood, even though it will appear disabled in the gRPC API.
- All published messages must (currently) have a JSON payload, not arbitrary
  binary data. This restriction may be lifted in the future.
- Some activities the Google emulator considers an error, `mmmbbb` does not. For
  example, several subscription settings can be modified via the gRPC API after
  subscription creation in `mmmbbb`, but not in Google's products. Google treats
  an HTTP Push endpoint that returns content with a 201 No Content response as
  an error (NACK), `mmmbbb` does not.

## Database model

The data model uses four tables: `topics`, `subscriptions`, `messages`, and
`deliveries`.

### Topics

Each PubSub topic is represented by a row in the `topics` table. When topics are
deleted, it is initially a "soft" delete, marking the row inactive. A background
process will eventually prune it once all the dependent rows in other tables
have been pruned as well.

In addition to helping performance, the background pruning model also allows
objects to be kept around for some time after they are deleted / completed for
debug / diagnostic inspections.

### Subscriptions

Each PubSub subscription is represented by a row in the `subscriptions` table.
Like with topics, deleting a subscription is initially a "soft" delete, followed
up by a background pruning process. Subscriptions are attached to topics by an
internal id, not by name, mirroring the behavior of the Google system.

### Messages

Every published message becomes a row in the `messages` table, linked to the
topic on which it was published.

### Deliveries

When a message is published, a row is added to the `deliveries` table for each
active subscription in the topic. Message ack/nack operations update the
corresponding row. Like with other entities, completed or expired deliveries are
not immediately removed, but instead get a "soft" marker, and are deleted later
by a background pruning process.

## Fault Injection

The application uses the fault injection "framework" from `gosix` to allow any
of the gRPC calls to have faults (errors) injected. The state of this is queried
via the `GET /faults` endpoint, and new injections can be added via the
`POST /faults/inject` endpoint. For more details on this, check the Swagger-UI
at `/oas-ui/`, or the [gosix
documentation](https://github.com/6RiverSystems/gosix/blob/main/docs/faults.md)
on the gRPC fault interceptor.

For example, to cause one call to the subscriber `StreamingPull` API to fail,
for a specific subscription, you could inject the following fault configuration:

```json
{
  "operation": "StreamingPull",
  "parameters": {
    "Subscription": "projects/foo/subscriptions/bar"
  },
  "count": 1
}
```
