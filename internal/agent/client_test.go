package agent

import (
	"errors"
	"net"
	"testing"
)

func TestIsTransientNetErr(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New(`Post "http://192.168.88.24:8001/v1/chat/completions": readfrom tcp 192.168.88.151:43618->192.168.88.24:8001: write tcp 192.168.88.151:43618->192.168.88.24:8001: write: broken pipe`), true},
		{errors.New("connection reset by peer"), true},
		{&net.DNSError{Err: "timeout", IsTimeout: true}, true},
		{errors.New("status code: 400, message: bad request"), false},
	}
	for _, tc := range cases {
		if got := isTransientNetErr(tc.err); got != tc.want {
			t.Errorf("isTransientNetErr(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
