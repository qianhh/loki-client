package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"gitlab.forceup.in/xlfs/loki-client/api"
	"gitlab.forceup.in/xlfs/loki-client/push"
	"golang.org/x/exp/slices"
)

const (
	errMaxStreamsLimitExceeded = "streams limit exceeded, streams: %d exceeds limit: %d, stream: '%s'"
)

// batch holds pending log streams waiting to be sent to Loki, and it's used
// to reduce the number of push requests to Loki aggregating multiple log streams
// and entries in a single batch request. In case of multi-tenant Promtail, log
// streams for each tenant are stored in a dedicated batch.
type batch struct {
	streams   map[string]*push.Stream
	bytes     int
	createdAt time.Time

	maxStreams int
}

func newBatch(maxStreams int, entries ...api.Entry) *batch {
	b := &batch{
		streams:    map[string]*push.Stream{},
		bytes:      0,
		createdAt:  time.Now(),
		maxStreams: maxStreams,
	}

	// Add entries to the batch
	for _, entry := range entries {
		//never error here
		_ = b.add(entry)
	}

	return b
}

// add an entry to the batch
func (b *batch) add(entry api.Entry) error {
	b.bytes += entrySize(entry)

	// Append the entry to an already existing stream (if any)
	labels := labelsMapToString(entry.Labels, ReservedLabelTenantID)
	if stream, ok := b.streams[labels]; ok {
		stream.Entries = append(stream.Entries, entry.Entry)
		return nil
	}

	streams := len(b.streams)
	if b.maxStreams > 0 && streams >= b.maxStreams {
		return fmt.Errorf(errMaxStreamsLimitExceeded, streams, b.maxStreams, labels)
	}
	// Add the entry as a new stream
	b.streams[labels] = &push.Stream{
		Labels:  labels,
		Entries: []push.Entry{entry.Entry},
	}
	return nil
}

func labelsMapToString(ls model.LabelSet, without model.LabelName) string {
	var b strings.Builder
	totalSize := 2
	lstrs := make([]model.LabelName, 0, len(ls))

	for l, v := range ls {
		if l == without {
			continue
		}

		lstrs = append(lstrs, l)
		// guess size increase: 2 for `, ` between labels and 3 for the `=` and quotes around label value
		totalSize += len(l) + 2 + len(v) + 3
	}

	b.Grow(totalSize)
	b.WriteByte('{')
	slices.Sort(lstrs)
	for i, l := range lstrs {
		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(string(l))
		b.WriteString(`=`)
		b.WriteString(strconv.Quote(string(ls[l])))
	}
	b.WriteByte('}')

	return b.String()
}

// sizeBytes returns the current batch size in bytes
func (b *batch) sizeBytes() int {
	return b.bytes
}

// sizeBytesAfter returns the size of the batch after the input entry
// will be added to the batch itself
func (b *batch) sizeBytesAfter(entry api.Entry) int {
	return b.bytes + entrySize(entry)
}

// age of the batch since its creation
func (b *batch) age() time.Duration {
	return time.Since(b.createdAt)
}

// encode the batch as snappy-compressed push request, and returns
// the encoded bytes and the number of encoded entries
func (b *batch) encode() ([]byte, int, error) {
	req, entriesCount := b.createPushRequest()
	buf, err := proto.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	buf = snappy.Encode(nil, buf)
	return buf, entriesCount, nil
}

// creates push request and returns it, together with number of entries
func (b *batch) createPushRequest() (*push.PushRequest, int) {
	req := push.PushRequest{
		Streams: make([]push.Stream, 0, len(b.streams)),
	}

	entriesCount := 0
	for _, stream := range b.streams {
		req.Streams = append(req.Streams, *stream)
		entriesCount += len(stream.Entries)
	}
	return &req, entriesCount
}

func entrySize(entry api.Entry) int {
	structuredMetadataSize := 0
	for _, label := range entry.StructuredMetadata {
		structuredMetadataSize += label.Size()
	}
	return len(entry.Line) + structuredMetadataSize
}
