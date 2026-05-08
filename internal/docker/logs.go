package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"strings"
)

// demuxLogs parses docker's multiplexed log stream:
//   header[0]   = stream ID (0=stdin, 1=stdout, 2=stderr)
//   header[1:4] = padding
//   header[4:8] = big-endian payload length
//   payload     = N bytes
//
// We read one frame at a time, split on '\n', and emit one LogLine per line.
// If the daemon was started with `tty: true`, there is no header — we fall
// back to plain line scanning when the first byte doesn't look like a header.
func demuxLogs(ctx context.Context, r io.Reader, out chan<- LogLine) {
	br := bufio.NewReader(r)

	first, err := br.Peek(1)
	if err != nil {
		return
	}
	// TTY logs have arbitrary first bytes; the multiplex header always has
	// stream ID in {0,1,2}. Bytes 3-7 of the header are padding zeros.
	if first[0] > 2 {
		scanLines(ctx, br, "stdout", out)
		return
	}

	header := make([]byte, 8)
	for {
		if ctx.Err() != nil {
			return
		}
		if _, err := io.ReadFull(br, header); err != nil {
			return
		}
		stream := streamName(header[0])
		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			continue
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(br, payload); err != nil {
			return
		}
		emitChunk(ctx, payload, stream, out)
	}
}

func scanLines(ctx context.Context, r io.Reader, stream string, out chan<- LogLine) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1*1024*1024)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return
		case out <- LogLine{Stream: stream, Line: sc.Text()}:
		}
	}
}

func emitChunk(ctx context.Context, payload []byte, stream string, out chan<- LogLine) {
	for _, line := range bytes.Split(payload, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case out <- LogLine{Stream: stream, Line: strings.TrimRight(string(line), "\r")}:
		}
	}
}

func streamName(b byte) string {
	switch b {
	case 1:
		return "stdout"
	case 2:
		return "stderr"
	default:
		return "stdin"
	}
}
