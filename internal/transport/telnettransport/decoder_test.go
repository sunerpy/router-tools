package telnettransport_test

import (
	"bytes"
	"testing"

	"github.com/sunerpy/router-tools/internal/transport/telnettransport"
)

func TestDecoderAcrossSingleByteChunks(t *testing.T) {
	t.Parallel()

	input := []byte{
		255, 251, 1, // WILL ECHO -> DO ECHO
		255, 253, 3, // DO SGA -> WILL SGA
		255, 251, 99, // unknown WILL -> DONT
		255, 250, 1, 255, 255, 2, 255, 240, // subnegotiation
		'A', 255, 255, 'B',
	}
	decoder := &telnettransport.Decoder{}
	var data []byte
	var responses []byte
	for _, value := range input {
		gotData, gotResponses := decoder.Push([]byte{value})
		data = append(data, gotData...)
		responses = append(responses, gotResponses...)
	}
	if !bytes.Equal(data, []byte{'A', 255, 'B'}) {
		t.Fatalf("decoded data = %v", data)
	}
	wantResponses := []byte{255, 253, 1, 255, 251, 3, 255, 254, 99}
	if !bytes.Equal(responses, wantResponses) {
		t.Fatalf("responses = %v, want %v", responses, wantResponses)
	}
}
