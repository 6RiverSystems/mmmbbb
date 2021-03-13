#!/bin/sh

# TODO: this should be production-y with all the self-labelling shenaningans
# from node_base

# hack: see notes on CMD in Dockerfile for what is going on here
if [ "$*" = '/app/${BINARYNAME}' ]; then
	exec "/app/${BINARYNAME}"
else
	exec "$@"
fi
