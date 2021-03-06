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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/m3db/m3cluster/client"
	"github.com/m3db/m3cluster/kv"
	etcdkv "github.com/m3db/m3cluster/kv/etcd"
	"github.com/m3db/m3cluster/services"
	etcdsd "github.com/m3db/m3cluster/services/client/etcd"
	etcdheartbeat "github.com/m3db/m3cluster/services/heartbeat/etcd"
	"github.com/m3db/m3cluster/services/leader"
	"github.com/m3db/m3x/instrument"
	"github.com/m3db/m3x/log"

	"github.com/coreos/etcd/clientv3"
	"github.com/uber-go/tally"
)

const (
	hierarchySeparator = "/"
	internalPrefix     = "_"
	cacheFileSeparator = "_"
	cacheFileSuffix    = ".json"
	// TODO deprecate this once all keys are migrated to per service namespace
	kvPrefix = "_kv"
)

var errInvalidNamespace = errors.New("invalid namespace")

type newClientFn func(cluster Cluster) (*clientv3.Client, error)

type cacheFileForZoneFn func(zone string) etcdkv.CacheFileFn

// NewConfigServiceClient returns a ConfigServiceClient
func NewConfigServiceClient(opts Options) (client.Client, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	scope := opts.InstrumentOptions().
		MetricsScope().
		Tagged(map[string]string{"service": opts.Service()})

	return &csclient{
		opts:    opts,
		sdOpts:  opts.ServiceDiscoveryConfig().NewOptions(),
		kvScope: scope.Tagged(map[string]string{"config_service": "kv"}),
		sdScope: scope.Tagged(map[string]string{"config_service": "sd"}),
		hbScope: scope.Tagged(map[string]string{"config_service": "hb"}),
		clis:    make(map[string]*clientv3.Client),
		logger:  opts.InstrumentOptions().Logger(),
		newFn:   newClient,
	}, nil
}

type csclient struct {
	sync.RWMutex
	clis map[string]*clientv3.Client

	opts    Options
	sdOpts  etcdsd.Options
	kvScope tally.Scope
	sdScope tally.Scope
	hbScope tally.Scope
	logger  log.Logger
	newFn   newClientFn

	txnOnce sync.Once
	txn     kv.TxnStore
	txnErr  error
}

func (c *csclient) Services(opts services.Options) (services.Services, error) {
	if opts == nil {
		opts = services.NewOptions()
	}
	return c.createServices(opts)
}

func (c *csclient) KV() (kv.Store, error) {
	return c.Txn()
}

func (c *csclient) Txn() (kv.TxnStore, error) {
	c.txnOnce.Do(func() {
		c.txn, c.txnErr = c.createTxnStore(
			kv.NewOptions().
				SetNamespace(kvPrefix).
				SetEnvironment(c.opts.Env()),
		)

	})

	return c.txn, c.txnErr
}

func (c *csclient) Store(opts kv.Options) (kv.Store, error) {
	return c.TxnStore(opts)
}

func (c *csclient) TxnStore(opts kv.Options) (kv.TxnStore, error) {
	opts, err := c.sanitizeOptions(opts)
	if err != nil {
		return nil, err
	}

	return c.createTxnStore(opts)
}

func (c *csclient) createServices(opts services.Options) (services.Services, error) {
	nOpts := opts.NamespaceOptions()
	cacheFileExtraFields := []string{nOpts.PlacementNamespace(), nOpts.MetadataNamespace()}
	return etcdsd.NewServices(c.sdOpts.
		SetHeartbeatGen(c.heartbeatGen()).
		SetKVGen(c.kvGen(c.cacheFileFn(cacheFileExtraFields...))).
		SetLeaderGen(c.leaderGen()).
		SetNamespaceOptions(nOpts).
		SetInstrumentsOptions(instrument.NewOptions().
			SetLogger(c.logger).
			SetMetricsScope(c.sdScope),
		),
	)
}

func (c *csclient) createTxnStore(opts kv.Options) (kv.TxnStore, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	return c.txnGen(c.opts.Zone(), c.cacheFileFn(), opts.Logger(), opts.Namespace(), opts.Environment())
}

func (c *csclient) kvGen(fn cacheFileForZoneFn) etcdsd.KVGen {
	return etcdsd.KVGen(func(zone string) (kv.Store, error) {
		return c.txnGen(zone, fn, c.logger)
	})
}

func (c *csclient) newkvOptions(
	zone string,
	cacheFileFn cacheFileForZoneFn,
	logger log.Logger,
	namespaces ...string,
) etcdkv.Options {
	opts := etcdkv.NewOptions().
		SetInstrumentsOptions(instrument.NewOptions().
			SetLogger(logger).
			SetMetricsScope(c.kvScope)).
		SetCacheFileFn(cacheFileFn(zone))

	for _, namespace := range namespaces {
		if namespace == "" {
			continue
		}
		opts = opts.SetPrefix(opts.ApplyPrefix(namespace))
	}
	return opts
}

func (c *csclient) txnGen(
	zone string,
	cacheFileFn cacheFileForZoneFn,
	logger log.Logger,
	namespaces ...string,
) (kv.TxnStore, error) {
	cli, err := c.etcdClientGen(zone)
	if err != nil {
		return nil, err
	}

	return etcdkv.NewStore(
		cli.KV,
		cli.Watcher,
		c.newkvOptions(zone, cacheFileFn, logger, namespaces...),
	)
}

func (c *csclient) heartbeatGen() etcdsd.HeartbeatGen {
	return etcdsd.HeartbeatGen(
		func(sid services.ServiceID) (services.HeartbeatService, error) {
			cli, err := c.etcdClientGen(sid.Zone())
			if err != nil {
				return nil, err
			}

			opts := etcdheartbeat.NewOptions().
				SetInstrumentsOptions(instrument.NewOptions().
					SetLogger(c.logger).
					SetMetricsScope(c.hbScope)).
				SetServiceID(sid)
			return etcdheartbeat.NewStore(cli, opts)
		},
	)
}

func (c *csclient) leaderGen() etcdsd.LeaderGen {
	return etcdsd.LeaderGen(
		func(sid services.ServiceID, eo services.ElectionOptions) (services.LeaderService, error) {
			cli, err := c.etcdClientGen(sid.Zone())
			if err != nil {
				return nil, err
			}

			opts := leader.NewOptions().
				SetServiceID(sid).
				SetElectionOpts(eo)

			return leader.NewService(cli, opts)
		},
	)
}

func (c *csclient) etcdClientGen(zone string) (*clientv3.Client, error) {
	c.Lock()
	defer c.Unlock()

	cli, ok := c.clis[zone]
	if ok {
		return cli, nil
	}

	cluster, ok := c.opts.ClusterForZone(zone)
	if !ok {
		return nil, fmt.Errorf("no etcd cluster found for zone %s", zone)
	}

	cli, err := c.newFn(cluster)
	if err != nil {
		return nil, err
	}

	c.clis[zone] = cli
	return cli, nil
}

func newClient(cluster Cluster) (*clientv3.Client, error) {
	tls, err := cluster.TLSOptions().Config()
	if err != nil {
		return nil, err
	}

	return clientv3.New(clientv3.Config{Endpoints: cluster.Endpoints(), TLS: tls})
}

func (c *csclient) cacheFileFn(extraFields ...string) cacheFileForZoneFn {
	return func(zone string) etcdkv.CacheFileFn {
		return func(namespace string) string {
			if c.opts.CacheDir() == "" {
				return ""
			}

			cacheFileFields := make([]string, 0, len(extraFields)+3)
			cacheFileFields = append(cacheFileFields, namespace, c.opts.Service(), zone)
			cacheFileFields = append(cacheFileFields, extraFields...)
			return filepath.Join(c.opts.CacheDir(), fileName(cacheFileFields...))
		}
	}
}

func fileName(parts ...string) string {
	// get non-empty parts
	idx := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i != idx {
			parts[idx] = part
		}
		idx++
	}
	parts = parts[:idx]
	s := strings.Join(parts, cacheFileSeparator)
	return strings.Replace(s, string(os.PathSeparator), cacheFileSeparator, -1) + cacheFileSuffix
}

func validateTopLevelNamespace(namespace string) error {
	if namespace == "" || namespace == hierarchySeparator {
		return errInvalidNamespace
	}
	if strings.HasPrefix(namespace, internalPrefix) {
		// start with _
		return errInvalidNamespace
	}
	if strings.HasPrefix(namespace, hierarchySeparator+internalPrefix) {
		return errInvalidNamespace
	}
	return nil
}

func (c *csclient) sanitizeOptions(opts kv.Options) (kv.Options, error) {
	if opts.Logger() == nil {
		opts = opts.SetLogger(c.logger)
	}

	if opts.Environment() == "" {
		opts = opts.SetEnvironment(c.opts.Env())
	}

	namespace := opts.Namespace()
	if namespace == "" {
		return opts.SetNamespace(kvPrefix), nil
	}

	return opts, validateTopLevelNamespace(namespace)
}
