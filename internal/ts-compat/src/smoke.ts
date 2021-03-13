import { PubSub, Message } from "@google-cloud/pubsub";

async function sleep(ms: number) {
  await new Promise((r) => setTimeout(r, ms));
}

function trimMsg(message: Message) {
  return {
    id: message.id,
    ackId: message.ackId,
    attributes: message.attributes,
    deliveryAttempt: message.deliveryAttempt,
    orderingKey: message.orderingKey,
    publishTime: new Date(message.publishTime),
    data: message.data.toString(),
  };
}

async function main() {
  process.env.PUBSUB_EMULATOR_HOST = "localhost:8802";
  const p = new PubSub({
    projectId: "ts-compat",
  });

  const [topics] = await p.getTopics();
  console.log("Num topics:", topics.length);
  const [subs] = await p.getSubscriptions();
  console.log("Num subs:", subs.length);

  const id = "ts-stress-" + Math.random().toString().substr(2);

  try {
    const [t] = await p.createTopic(id);
    await p.createTopic(id);
  } catch (err) {
    // ignore already exists
    if (err.code !== 6) {
      console.error(err);
    }
  }

  const [t] = await p.topic(id).get({ autoCreate: true });
  // while gRPC level allows creating a topic with labels, NodeJS client doesn't
  // seem to expose this
  await t.setMetadata({
    labels: {
      ...t.metadata?.labels,
      b: Date.now().toString(),
    },
  });

  const [s] = await t.subscription(id).get({ autoCreate: true });

  // while gRPC level allows creating a sub with labels, NodeJS client doesn't
  // seem to expose this
  await s.setMetadata({
    enableMessageOrdering: true,
    labels: {
      ...s.metadata?.labels,
      b: Date.now().toString(),
    },
    retryPolicy: {
      minimumBackoff: { seconds: 1, nanos: 5e8 },
    },
  });

  let msgTimeout: NodeJS.Timeout;
  let msgsP = new Promise((r) => {
    msgTimeout = setTimeout(r, 1100);
  });

  let gotError = false;
  s.on("error", (err) => {
    gotError = true;
    console.error(err);
  });
  s.on("message", async (message: Message) => {
    console.log("received", trimMsg(message));
    msgTimeout.refresh();
    await sleep(1000 * Math.random());
    console.log("ack", trimMsg(message));
    message.ack();
    msgTimeout.refresh();
  });

  const msgID = await t.publishJSON(
    { hello: "world" },
    { attr1: new Date().toISOString() }
  );
  console.log("Published messageID:", msgID);

  await msgsP;

  console.log("deleting sub, expect 'Not found' error:");
  await t.subscription(id).delete();
  await sleep(100);
  console.log("got expected error?", gotError);
  await t.delete();

  await s.close();
  await p.close();
}

main().catch((err) => console.log(err));
