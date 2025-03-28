# mmmbbb

A Message Bus for 6RiverSystems.

This is, in fact, a near drop-in replacement for the Google PubSub emulator.
Specifically it implements most of the PubSub gRPC API, and specifically a
superset of what the emulator does, sufficient for the needs of 6RS.

## Running it

### For Rivs

This will be run automatically by `6mon`.

### For others

You can build and run the `mmmbbb` app locally, or you can use the bundled
[ko](https://ko.build/) configuration to build a docker image.

The docker image defaults to using a SQLite back-end, which is stored
ephemerally in `/tmp` in the container. This is done to minimize the amount of
setup required, but it should be noted that the PostgreSQL backend has
considerably better performance in addition to allowing data to persist across
container instantiations.

In its simplest form, you can use:

```shell
docker run --rm --publish 8084-8085:8084-8085/tcp 6river/mmmbbb
```

### Initializing a PostgreSQL Database

The application will create all the tables it needs automatically, as long as it
can connect to the database.

You can further simplify setup by including the `CREATE_DB_VIA` environment
variable. If the `DATABASE_URL` is otherwise valid, but the target database
doesn't exist, and this environment variable is set, the app will try to connect
to this alternate database and issue a `CREATE DATABASE` for its target
database. If this succeeds, it will switch back to its own database and continue
with startup.

## Configuration

### Environment variables

- `NODE_ENV`
  - Useful values are `test`, `development`, or `production`
  - Determines defaults for logging and the database to use
- `DATABASE_URL`
  - Supported values are `postgres://user:password@host:port/database` or
    `sqlite:///path/to/file.sqlite` (note the number of slashes!). See below for
    extra notes for using SQLite.
- `LOG_LEVEL`
  - Supported values are `trace`, `debug`, `info`, `warn`, `error`, `fatal`,
    `panic` (i.e. the level names from `zerolog`)
- `PORT`
  - Sets the _base_ port for the application. This is where it listens for
    normal HTTP connections for metrics and the OAS UI.
  - The gRPC port will be this plus one
- `CREATE_DB_VIA`
  - See above regarding PostgreSQL initialization

### Supported persistence backends

Both PostgreSQL and SQLite are supported backends. With SQLite, only file-based
storage is currently supported, not memory-based storage, due to issues with
concurrency, locking, and some implementation issues with the Go SQLite drivers.

Which SQLite driver is used depends on whether `mmmbbb` was compiled with CGo
enabled or not. When enabled, `github.com/mattn/go-sqlite3` is used. When CGo is
disabled, `modernc.org/sqlite` is used.

To select a backend, set the `DATABASE_URL` appropriately (see above on
environment variables).

SQLite as a backend currently requires explicitly specifying several extra
parameters in the `DATABASE_URL`.

CGo enabled (`mattn` driver, only enabled if building from source):

```text
?_fk=true&_journal_mode=wal&cache=private&_busy_timeout=10000&_txlock=immediate
```

CGo disabled (`modernc` driver, this is what `ko`-built containers will use):

```text
?_pragma=foreign_keys(1)&_pragma=journal_mode(wal)&_pragma=busy_timeout(10000)&_txlock=immediate
```

In both cases, shell escaping is almost certainly needed when entering these
values!

## What is and isn't implemented

The bulk of the gRPC API is implemented. If your application only sends JSON
payloads, and works with the emulator Google provides as part of the Google
Cloud SDK as of June 2021, it should work with this one too.

### Supported Features Beyond Google's Emulator

Several features are supported here that Google's emulator does not support (as
of June 2021):

- Server driven delivery backoff (but see note below on how this differs from
  Google's production implementation)
- Subscription filters
- Dead letter topics for subscriptions
- Ordered message delivery to subscriptions

### Features that are not implemented

- Region settings
- KMS settings
- Custom ACK deadlines
- ACK'd message retention
  - While you cannot configure this, there is a limited retention that is always
    active, see below on the partial support for seek to time
- Authenticating PUSH delivery
- The Schema API and any PubSub settings related to it
- Authentication settings for Push subscriptions
- Authenticating to the service itself
- Detaching subscriptions
- Updating snapshots
- Changing message retry backoff settings for a subscription may not fully take
  effect when actively streaming messages from it (including from an HTTP Push
  configuration)

### Features that are different

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
- This is a clean room implementation, based only on _documented_ behavior. As
  such, some corner cases where Google does not describe what their system does
  in their API documentation may be have differently with this implementation.
  - Example: Google does not currently document what happens if you delete a
    topic that is referenced as the dead letter topic for some Subscription.
- Seeking a subscription to a time or snapshot doesn't require enabling message
  retention, since `mmmbbb` always retains messages to a limited extent. While
  seeking to a time outside the limited automatic retention will not produce an
  error, neither will it resurrect messages that have been permanently deleted.

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

## Delivery Delays

For testing purposes, a subscription can be configured to delay delivering all
messages a minimum time after they were published. Open the `/oas-ui/` and look
at the `/delays` endpoints for details. The gist is that every message for the
subscription will not have any delivery attempted until the given delay after it
was published. This delay does _not_ adjust for prior messages in an ordered
delivery subscription, it is just a minimum initial delay after the initial
publish.

### Examples

To cause one call to the subscriber `StreamingPull` API to fail,
for a specific subscription, you could inject the following fault configuration:

```json
{
  "operation": "StreamingPull",
  "parameters": {
    "subscription": "projects/foo/subscriptions/bar"
  },
  "count": 1
}
```

To cause one call to the `Publish` API to fail, for a specific topic,
you could inject the following fault configuration:

```json
{
  "operation": "Publish",
  "parameters": {
    "topic": "projects/foo/topics/bar"
  },
  "count": 1
}
```
