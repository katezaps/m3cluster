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

package placement

import (
	"encoding/json"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSnapshot(t *testing.T) {
	h1 := NewHostShards(NewHost("r1h1", "r1", "z1", 1))
	h1.AddShard(1)
	h1.AddShard(2)
	h1.AddShard(3)

	h2 := NewHostShards(NewHost("r2h2", "r2", "z1", 1))
	h2.AddShard(4)
	h2.AddShard(5)
	h2.AddShard(6)

	h3 := NewHostShards(NewHost("r3h3", "r3", "z1", 1))
	h3.AddShard(1)
	h3.AddShard(3)
	h3.AddShard(5)

	h4 := NewHostShards(NewHost("r4h4", "r4", "z1", 1))
	h4.AddShard(2)
	h4.AddShard(4)
	h4.AddShard(6)

	h5 := NewHostShards(NewHost("r5h5", "r5", "z1", 1))
	h5.AddShard(5)
	h5.AddShard(6)
	h5.AddShard(1)

	h6 := NewHostShards(NewHost("r6h6", "r6", "z1", 1))
	h6.AddShard(2)
	h6.AddShard(3)
	h6.AddShard(4)

	hss := []HostShards{h1, h2, h3, h4, h5, h6}

	ids := []uint32{1, 2, 3, 4, 5, 6}
	s := NewPlacementSnapshot(hss, ids, 3)
	assert.NoError(t, s.Validate())
	testSnapshotJSONRoundTrip(t, s)

	hs := s.HostShard("r6h6")
	assert.Equal(t, h6, hs)
	hs = s.HostShard("h100")
	assert.Nil(t, hs)

	assert.Equal(t, 6, s.HostsLen())
	assert.Equal(t, 3, s.Replicas())
	assert.Equal(t, ids, s.Shards())
	assert.Equal(t, hss, s.HostShards())

	s = NewEmptyPlacementSnapshot([]Host{NewHost("h1", "r1", "z1", 1), NewHost("h2", "r2", "z1", 1)}, ids)
	assert.Equal(t, 0, s.Replicas())
	assert.Equal(t, ids, s.Shards())
	assert.NoError(t, s.Validate())
}

func TestValidate(t *testing.T) {
	ids := []uint32{1, 2, 3, 4, 5, 6}

	h1 := NewHostShards(NewHost("r1h1", "r1", "z1", 1))
	h1.AddShard(1)
	h1.AddShard(2)
	h1.AddShard(3)

	h2 := NewHostShards(NewHost("r2h2", "r2", "z1", 1))
	h2.AddShard(4)
	h2.AddShard(5)
	h2.AddShard(6)

	hss := []HostShards{h1, h2}
	s := NewPlacementSnapshot(hss, ids, 1)
	assert.NoError(t, s.Validate())

	// mismatch shards
	s = NewPlacementSnapshot(hss, append(ids, 7), 1)
	assert.Error(t, s.Validate())
	assert.Error(t, s.Validate())

	// host missing a shard
	h1 = NewHostShards(NewHost("r1h1", "r1", "z1", 1))
	h1.AddShard(1)
	h1.AddShard(2)
	h1.AddShard(3)
	h1.AddShard(4)
	h1.AddShard(5)
	h1.AddShard(6)

	h2 = NewHostShards(NewHost("r2h2", "r2", "z1", 1))
	h2.AddShard(2)
	h2.AddShard(3)
	h2.AddShard(4)
	h2.AddShard(5)
	h2.AddShard(6)

	hss = []HostShards{h1, h2}
	s = NewPlacementSnapshot(hss, ids, 2)
	assert.Error(t, s.Validate())
	assert.Equal(t, errTotalShardsMismatch, s.Validate())

	// host contains shard that's unexpected to be in snapshot
	h1 = NewHostShards(NewHost("r1h1", "r1", "z1", 1))
	h1.AddShard(1)
	h1.AddShard(2)
	h1.AddShard(3)
	h1.AddShard(4)
	h1.AddShard(5)
	h1.AddShard(6)
	h1.AddShard(7)

	h2 = NewHostShards(NewHost("r2h2", "r2", "z1", 1))
	h2.AddShard(2)
	h2.AddShard(3)
	h2.AddShard(4)
	h2.AddShard(5)
	h2.AddShard(6)

	hss = []HostShards{h1, h2}
	s = NewPlacementSnapshot(hss, ids, 2)
	assert.Error(t, s.Validate())
	assert.Equal(t, errUnexpectedShards, s.Validate())

	// duplicated shards
	h1 = NewHostShards(NewHost("r1h1", "r1", "z1", 1))
	h1.AddShard(2)
	h1.AddShard(3)
	h1.AddShard(4)

	h2 = NewHostShards(NewHost("r2h2", "r2", "z1", 1))
	h2.AddShard(4)
	h2.AddShard(5)
	h2.AddShard(6)

	hss = []HostShards{h1, h2}
	s = NewPlacementSnapshot(hss, []uint32{2, 3, 4, 4, 5, 6}, 1)
	assert.Error(t, s.Validate())
	assert.Equal(t, errDuplicatedShards, s.Validate())

	// three shard 2 and only one shard 4
	h1 = NewHostShards(NewHost("r1h1", "r1", "z1", 1))
	h1.AddShard(1)
	h1.AddShard(2)
	h1.AddShard(3)

	h2 = NewHostShards(NewHost("r2h2", "r2", "z1", 1))
	h2.AddShard(2)
	h2.AddShard(3)
	h2.AddShard(4)

	h3 := NewHostShards(NewHost("r3h3", "r3", "z1", 1))
	h3.AddShard(1)
	h3.AddShard(2)

	hss = []HostShards{h1, h2, h3}
	s = NewPlacementSnapshot(hss, []uint32{1, 2, 3, 4}, 2)
	assert.Error(t, s.Validate())
}

func TestSnapshotMarshalling(t *testing.T) {
	invalidJSON := `{
		"abc":{"ID":123,"Rack":"r1","Zone":"z1","Weight":50,"Shards":[0,7,11]}
	}`
	data := []byte(invalidJSON)
	ps, err := NewPlacementFromJSON(data)
	assert.Nil(t, ps)
	assert.Error(t, err)

	ps, err = NewPlacementFromJSON([]byte(`{
		"h1":{"ID":"h1","Rack":"r1","Zone":"z1","Weight":50,"Shards":[0,7,11]}
	}`))
	hs := ps.HostShard("h1")
	assert.NotNil(t, hs)
	assert.Equal(t, "[id:h1, rack:r1, zone:z1, weight:50]", hs.Host().String())
	assert.Equal(t, 3, hs.ShardsLen())
	assert.Equal(t, map[uint32]struct{}{0: struct{}{}, 7: struct{}{}, 11: struct{}{}}, hs.(*hostShards).shardsSet)

	validJSON := `{
		"r2h4": {"ID":"r2h4","Rack":"r2","Zone":"z1","Weight":50,"Shards":[6,13,15]},
		"r3h5": {"ID":"r3h5","Rack":"r3","Zone":"z1","Weight":50,"Shards":[2,8,19]},
		"r4h6": {"ID":"r4h6","Rack":"r4","Zone":"z1","Weight":50,"Shards":[3,9,18]},
		"r1h1": {"ID":"r1h1","Rack":"r1","Zone":"z1","Weight":50,"Shards":[0,7,11]},
		"r2h3": {"ID":"r2h3","Rack":"r2","Zone":"z1","Weight":50,"Shards":[1,4,12]},
		"r5h7": {"ID":"r5h7","Rack":"r5","Zone":"z1","Weight":50,"Shards":[10,14]},
		"r6h9": {"ID":"r6h9","Rack":"r6","Zone":"z1","Weight":50,"Shards":[5,16,17]}
	}`
	data = []byte(validJSON)
	ps, err = NewPlacementFromJSON(data)
	assert.NoError(t, err)
	assert.Equal(t, 7, ps.HostsLen())
	assert.Equal(t, 1, ps.Replicas())
	assert.Equal(t, 20, ps.ShardsLen())

	testSnapshotJSONRoundTrip(t, ps)

	// an extra replica for shard 1
	invalidPlacementJSON := `{
		"r1h1": {"ID":"r1h1","Rack":"r1","Zone":"z1","Weight":50,"Shards":[0,1,7,11]},
		"r2h3": {"ID":"r2h3","Rack":"r2","Zone":"z1","Weight":50,"Shards":[1,4,12]},
		"r2h4": {"ID":"r2h4","Rack":"r2","Zone":"z1","Weight":50,"Shards":[6,13,15]},
		"r3h5": {"ID":"r3h5","Rack":"r3","Zone":"z1","Weight":50,"Shards":[2,8,19]},
		"r4h6": {"ID":"r4h6","Rack":"r4","Zone":"z1","Weight":50,"Shards":[3,9,18]},
		"r5h7": {"ID":"r5h7","Rack":"r5","Zone":"z1","Weight":50,"Shards":[10,14]},
		"r6h9": {"ID":"r6h9","Rack":"r6","Zone":"z1","Weight":50,"Shards":[5,16,17]}
	}`
	data = []byte(invalidPlacementJSON)
	ps, err = NewPlacementFromJSON(data)
	assert.Equal(t, err, errInvalidShardsCount)
	assert.Nil(t, ps)

	// an extra replica for shard 0 on r1h1
	invalidPlacementJSON = `{
		"r1h1": {"ID":"r1h1","Rack":"r1","Zone":"z1","Weight":50,"Shards":[0,0,7,11]},
		"r2h3": {"ID":"r2h3","Rack":"r2","Zone":"z1","Weight":50,"Shards":[1,4,12]},
		"r2h4": {"ID":"r2h4","Rack":"r2","Zone":"z1","Weight":50,"Shards":[6,13,15]},
		"r3h5": {"ID":"r3h5","Rack":"r3","Zone":"z1","Weight":50,"Shards":[2,8,19]},
		"r4h6": {"ID":"r4h6","Rack":"r4","Zone":"z1","Weight":50,"Shards":[3,9,18]},
		"r5h7": {"ID":"r5h7","Rack":"r5","Zone":"z1","Weight":50,"Shards":[10,14]},
		"r6h9": {"ID":"r6h9","Rack":"r6","Zone":"z1","Weight":50,"Shards":[5,16,17]}
	}`
	data = []byte(invalidPlacementJSON)
	ps, err = NewPlacementFromJSON(data)
	assert.Equal(t, err, errInvalidHostShards)
	assert.Nil(t, ps)
}

func TestHostShards(t *testing.T) {
	h1 := NewHostShards(NewHost("r1h1", "r1", "z1", 1))
	h1.AddShard(1)
	h1.AddShard(2)
	h1.AddShard(3)

	assert.Equal(t, "[id:r1h1, rack:r1, zone:z1, weight:1]", h1.Host().String())

	assert.True(t, h1.ContainsShard(1))
	assert.False(t, h1.ContainsShard(100))
	assert.Equal(t, 3, h1.ShardsLen())
	assert.Equal(t, "r1h1", h1.Host().ID())
	assert.Equal(t, "r1", h1.Host().Rack())

	h1.RemoveShard(1)
	assert.False(t, h1.ContainsShard(1))
	assert.False(t, h1.ContainsShard(100))
	assert.Equal(t, 2, h1.ShardsLen())
	assert.Equal(t, "r1h1", h1.Host().ID())
	assert.Equal(t, "r1", h1.Host().Rack())
}

func TestCopy(t *testing.T) {
	h1 := NewHostShards(NewHost("r1h1", "r1", "z1", 1))
	h1.AddShard(1)
	h1.AddShard(2)
	h1.AddShard(3)

	h2 := NewHostShards(NewHost("r2h2", "r2", "z1", 1))
	h2.AddShard(4)
	h2.AddShard(5)
	h2.AddShard(6)

	hss := []HostShards{h1, h2}

	ids := []uint32{1, 2, 3, 4, 5, 6}
	s := NewPlacementSnapshot(hss, ids, 1)
	copy := s.Copy()
	assert.Equal(t, s.HostsLen(), copy.HostsLen())
	assert.Equal(t, s.Shards(), copy.Shards())
	assert.Equal(t, s.Replicas(), copy.Replicas())
	for _, hs := range s.HostShards() {
		assert.Equal(t, copy.HostShard(hs.Host().ID()), hs)
		// make sure they are different objects, updating one won't update the other
		hs.AddShard(100)
		assert.NotEqual(t, copy.HostShard(hs.Host().ID()), hs)
	}
}

func TestSortHostByID(t *testing.T) {
	h1 := NewHost("h1", "", "", 1)
	h2 := NewHost("h2", "", "", 1)
	h3 := NewHost("h3", "", "", 1)
	h4 := NewHost("h4", "", "", 1)
	h5 := NewHost("h5", "", "", 1)
	h6 := NewHost("h6", "", "", 1)

	hs := []Host{h1, h6, h4, h2, h3, h5}
	sort.Sort(ByIDAscending(hs))

	assert.Equal(t, []Host{h1, h2, h3, h4, h5, h6}, hs)
}

func TestOptions(t *testing.T) {
	o := NewOptions()
	assert.False(t, o.LooseRackCheck())
	assert.False(t, o.AcrossZones())
	assert.False(t, o.AllowPartialReplace())
	o = o.SetLooseRackCheck(true)
	assert.True(t, o.LooseRackCheck())
	o = o.SetAcrossZones(true)
	assert.True(t, o.AcrossZones())
	o = o.SetAllowPartialReplace(true)
	assert.True(t, o.AllowPartialReplace())
}

func testSnapshotJSONRoundTrip(t *testing.T, s Snapshot) {
	json1, err := json.Marshal(s)
	assert.NoError(t, err)

	var unmarshalled snapshot
	err = json.Unmarshal(json1, &unmarshalled)
	assert.NoError(t, err)

	assert.Equal(t, s.(snapshot).placementSnapshotToJSON(), unmarshalled.placementSnapshotToJSON())

}
