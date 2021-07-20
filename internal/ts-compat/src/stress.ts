/**
 * Copyright (c) 2021 6 River Systems
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of
 * this software and associated documentation files (the "Software"), to deal in
 * the Software without restriction, including without limitation the rights to
 * use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
 * the Software, and to permit persons to whom the Software is furnished to do so,
 * subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
 * FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
 * COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
 * IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
 * CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

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
