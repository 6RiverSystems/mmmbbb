FROM debian:stable-slim
LABEL MAINTAINER="Matthew Gabeler-Lee <mgabeler-lee@6river.com>"

ENTRYPOINT ["/usr/bin/dumb-init", "--", "/app/entrypoint.sh"]

RUN \
	apt-get update && \
	apt-get -y install dumb-init && \
	apt-get clean && \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# TODO: don't use NODE_ENV for non-NodeJS code
ENV NODE_ENV=production
# Apps will need DATABASE_URL set externally to a useful value

RUN mkdir -p /app
WORKDIR /app

COPY .docker-deps/entrypoint.sh /app/

# this will default to the env var at build time
ARG BINARYNAME=${BINARYNAME}
# this makes it available at runtime
ENV BINARYNAME ${BINARYNAME}
# This is actually getting passed to the entrypoint as the _literal_ string (no
# interpolation), and the entrypoint recognizes this magic value and does the
# right thing with the runtime value of BINARYNAME
CMD ["/app/${BINARYNAME}"]

# this uses the build-time arg/env
COPY bin/${BINARYNAME} /app/${BINARYNAME}
