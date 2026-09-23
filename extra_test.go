package datacounter_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"golift.io/datacounter"
)

type oneByteReader struct {
	rest []byte
}

func (r *oneByteReader) Read(buf []byte) (int, error) {
	if len(r.rest) == 0 {
		return 0, io.EOF
	}

	if len(buf) == 0 {
		return 0, nil
	}

	buf[0] = r.rest[0]
	r.rest = r.rest[1:]

	return 1, nil
}

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

type negReader struct{}

func (negReader) Read([]byte) (int, error) {
	return -1, io.ErrUnexpectedEOF
}

func TestReaderShortReadsAndNegativeCount(t *testing.T) {
	t.Parallel()

	data := []byte("Hello")
	counter := datacounter.NewReaderCounter(&oneByteReader{rest: data})

	got, err := io.ReadAll(counter)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, data) {
		t.Fatalf("read %q", got)
	}

	if counter.Count() != uint64(len(data)) {
		t.Fatalf("count %d", counter.Count())
	}

	if counter.Reads() < uint64(len(data)) {
		t.Fatalf("reads %d", counter.Reads())
	}

	neg := datacounter.NewReaderCounter(negReader{})
	if _, err = neg.Read(make([]byte, 4)); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("neg err %v", err)
	}

	if neg.Count() != 0 || neg.Reads() != 1 {
		t.Fatalf("neg count %d reads %d", neg.Count(), neg.Reads())
	}
}

func TestWriterError(t *testing.T) {
	t.Parallel()

	counter := datacounter.NewWriterCounter(errWriter{})

	_, err := counter.Write([]byte("nope"))
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("err %v", err)
	}

	if counter.Count() != 0 || counter.Writes() != 1 {
		t.Fatalf("count %d writes %d", counter.Count(), counter.Writes())
	}
}

func TestResponseWriterHeaderAndHijack(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	counter := datacounter.NewResponseWriterCounter(rec)
	counter.WriteHeader(http.StatusCreated)

	if rec.Code != http.StatusCreated {
		t.Fatalf("code %d", rec.Code)
	}

	if rec.Header().Get("X-Runtime") == "" {
		t.Fatal("missing X-Runtime")
	}

	if counter.StatusCode() != http.StatusCreated {
		t.Fatalf("status %d", counter.StatusCode())
	}

	if _, _, err := counter.Hijack(); err == nil {
		t.Fatal("expected hijack error")
	}
}
