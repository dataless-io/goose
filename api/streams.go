package api

import (
	"context"

	"github.com/fulldump/box"

	"goose/streams"
)

func InjectStreams(streams *streams.Streams) box.I {
	return func(next box.H) box.H {
		return func(ctx context.Context) {
			ctx = SetStreams(ctx, streams)
			next(ctx)
		}
	}
}

const StreamsKey = "251e85c6-7ce6-11ed-a744-c7b81561f85c"

func SetStreams(ctx context.Context, streams *streams.Streams) context.Context {
	return context.WithValue(ctx, StreamsKey, streams)
}

func GetStreams(ctx context.Context) *streams.Streams {
	return ctx.Value(StreamsKey).(*streams.Streams) // this panics if client is not present
}
