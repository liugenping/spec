// Copyright 2015 The appc Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package discovery

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"testing"
)

var testAuthHeader http.Header = http.Header{"Authorization": {"Basic YmFyOmJheg=="}}

// mockHttpDoer defines a wrapper that allows returning a mocked response.
type mockHttpDoer struct {
	doer func(req *http.Request) (resp *http.Response, err error)
}

func (m *mockHttpDoer) Do(req *http.Request) (resp *http.Response, err error) {
	return m.doer(req)
}

func fakeHttpOrHttpsGet(filename string, httpSuccess bool, httpsSuccess bool, httpErrorCode int, header http.Header) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}

		var resp *http.Response

		if header != nil && !reflect.DeepEqual(req.Header, header) {
			err = fmt.Errorf("fakeHttpOrHttpsGet: wrong header %v. Expected %v", req.Header, header)
			return nil, err
		}

		switch {
		case req.URL.Scheme == "https" && httpsSuccess:
			fallthrough
		case req.URL.Scheme == "http" && httpSuccess:
			resp = &http.Response{
				Status:     "200 OK",
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: f,
			}
		case httpErrorCode > 0:
			resp = &http.Response{
				Status:     "Error",
				StatusCode: httpErrorCode,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header: http.Header{
					"Content-Type": []string{"text/html"},
				},
				Body: ioutil.NopCloser(bytes.NewBufferString("")),
			}
		default:
			err = errors.New("fakeHttpOrHttpsGet failed as requested")
			return nil, err
		}

		return resp, nil
	}
}

func TestHttpsOrHTTP(t *testing.T) {
	tests := []struct {
		name          string
		insecure      bool
		do            httpDoer
		expectUrlStr  string
		expectSuccess bool
		authHeader    http.Header
	}{
		{
			"good-server",
			false,
			&mockHttpDoer{
				doer: fakeHttpOrHttpsGet("myapp.html", true, true, 0, nil),
			},
			"https://good-server?ac-discovery=1",
			true,
			nil,
		},
		{
			"file-not-found",
			false,
			&mockHttpDoer{
				doer: fakeHttpOrHttpsGet("myapp.html", false, false, 404, nil),
			},
			"",
			false,
			nil,
		},
		{
			"completely-broken-server",
			false,
			&mockHttpDoer{
				doer: fakeHttpOrHttpsGet("myapp.html", false, false, 0, nil),
			},
			"",
			false,
			nil,
		},
		{
			"file-only-on-http",
			false, // do not accept fallback on http
			&mockHttpDoer{
				doer: fakeHttpOrHttpsGet("myapp.html", true, false, 404, nil),
			},
			"",
			false,
			nil,
		},
		{
			"file-only-on-http",
			true, // accept fallback on http
			&mockHttpDoer{
				doer: fakeHttpOrHttpsGet("myapp.html", true, false, 404, nil),
			},
			"http://file-only-on-http?ac-discovery=1",
			true,
			nil,
		},
		{
			"https-server-is-down",
			true, // accept fallback on http
			&mockHttpDoer{
				doer: fakeHttpOrHttpsGet("myapp.html", true, false, 0, nil),
			},
			"http://https-server-is-down?ac-discovery=1",
			true,
			nil,
		},
		{
			"coreos.com",
			false,
			&mockHttpDoer{
				doer: fakeHttpOrHttpsGet("myapp.html", false, true, 0, testAuthHeader),
			},
			"https://coreos.com?ac-discovery=1",
			true,
			testAuthHeader,
		},
	}

	for i, tt := range tests {
		httpDo = tt.do
		hostHeaders := map[string]http.Header{
			tt.name: tt.authHeader,
		}
		urlStr, body, err := httpsOrHTTP(tt.name, hostHeaders, tt.insecure)
		if tt.expectSuccess {
			if err != nil {
				t.Fatalf("#%d httpsOrHTTP failed: %v", i, err)
			}
			if urlStr == "" {
				t.Fatalf("#%d httpsOrHTTP didn't return a urlStr", i)
			}
			if urlStr != tt.expectUrlStr {
				t.Fatalf("#%d httpsOrHTTP urlStr mismatch: want %s got %s",
					i, tt.expectUrlStr, urlStr)
			}
			if body == nil {
				t.Fatalf("#%d httpsOrHTTP didn't return a body", i)
			}
		} else {
			if err == nil {
				t.Fatalf("#%d httpsOrHTTP should have failed", i)
			}
			if urlStr != "" {
				t.Fatalf("#%d httpsOrHTTP should not have returned a urlStr", i)
			}
			if body != nil {
				t.Fatalf("#%d httpsOrHTTP should not have returned a body", i)
			}
		}
	}
}
