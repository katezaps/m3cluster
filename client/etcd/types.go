// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package etcd

import (
	"crypto/tls"

	etcdsd "github.com/m3db/m3cluster/services/client/etcd"
	"github.com/m3db/m3x/instrument"
)

// Options is the Options to create a config service client.
type Options interface {
	Env() string
	SetEnv(e string) Options

	Zone() string
	SetZone(z string) Options

	Service() string
	SetService(id string) Options

	CacheDir() string
	SetCacheDir(dir string) Options

	ServiceDiscoveryConfig() etcdsd.Configuration
	SetServiceDiscoveryConfig(cfg etcdsd.Configuration) Options

	Clusters() []Cluster
	SetClusters(clusters []Cluster) Options
	ClusterForZone(z string) (Cluster, bool)

	InstrumentOptions() instrument.Options
	SetInstrumentOptions(iopts instrument.Options) Options

	Validate() error
}

// TLSOptions defines the configuration for TLS.
type TLSOptions interface {
	CrtPath() string
	SetCrtPath(string) TLSOptions

	KeyPath() string
	SetKeyPath(string) TLSOptions

	CACrtPath() string
	SetCACrtPath(string) TLSOptions

	Config() (*tls.Config, error)
}

// Cluster defines the configuration for a etcd cluster.
type Cluster interface {
	Zone() string
	SetZone(z string) Cluster

	Endpoints() []string
	SetEndpoints(endpoints []string) Cluster

	TLSOptions() TLSOptions
	SetTLSOptions(TLSOptions) Cluster
}
