import { PubSub, Message } from "@google-cloud/pubsub";

async function main() {
  process.env.PUBSUB_EMULATOR_HOST = "localhost:8802";
  const p = new PubSub({
    projectId: "ts-compat",
  });

  const id = "ts-stress-" + Math.random().toString().substr(2);

  const [t] = await p.topic(id).get({ autoCreate: true });
  const [s] = await t.subscription(id).get({ autoCreate: true });

  // configure the sub & its flow control same as our typical apps
  await s.setMetadata({
    enableMessageOrdering: false,
    retryPolicy: {
      minimumBackoff: { seconds: 1, nanos: 5e8 },
    },
  });
  s.setOptions({
    flowControl: {
      maxMessages: 20,
    },
  });

  const numMessages = 5000;
  let numSent = 0;
  let numReceived = 0;
  let done: () => void;
  let failed: (err: unknown) => void;
  let doneP = new Promise<void>((res, rej) => ([done, failed] = [res, rej]));

  s.on("error", (err) => {
    failed(err);
  });
  s.on("message", async (message: Message) => {
    ++numReceived;
    message.ack();
    if (numReceived >= numMessages) {
      console.log("all received");
      done();
    } else if (numReceived % 1000 === 0) {
      console.log(
        "received",
        numReceived,
        (Date.now() - start) / numReceived,
        "ms/msg"
      );
    }
  });

  const payload = { hello: "world" };

  const start = Date.now();
  const publishP = Promise.all(
    Array.from({ length: numMessages }, async (i) => {
      // don't start these until the parent routine gets going
      await Promise.resolve();
      await t.publishJSON(payload);
      ++numSent;
      if (numSent >= numMessages) {
        console.log("all sent");
      } else if (numSent % 1000 === 0) {
        console.log("sent", numSent);
      }
    })
  );

  try {
    await Promise.all([publishP, doneP]);
    const end = Date.now();
    const duration = end - start;
    console.log("took", duration, "ms");
    console.log(duration / numMessages, "ms/msg");
  } catch (err) {
    console.error(err);
  } finally {
    await Promise.all([s.delete(), t.delete()]);
  }
  console.log("done", numMessages, numSent, numReceived);
}

main().catch((err) => console.log(err));
