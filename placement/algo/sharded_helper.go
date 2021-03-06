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

package algo

import (
	"container/heap"
	"errors"
	"fmt"
	"math"

	"github.com/m3db/m3cluster/placement"
	"github.com/m3db/m3cluster/shard"
	"github.com/m3db/m3x/log"
)

var (
	errAddingInstanceAlreadyExist         = errors.New("the adding instance is already in the placement")
	errInstanceContainsNonLeavingShards   = errors.New("the adding instance contains non leaving shards")
	errInstanceContainsInitializingShards = errors.New("the adding instance contains initializing shards")
)

type instanceType int

const (
	anyType instanceType = iota
	withShards
	withLeavingShardsOnly
	withAvailableOrLeavingShardsOnly
)

type optimizeType int

const (
	// safe optimizes the load distribution without violating
	// minimal shard movemoment.
	safe optimizeType = iota
	// unsafe optimizes the load distribution with the potential of violating
	// minimal shard movement in order to reach best shard distribution
	unsafe
)

type assignLoadFn func(instance placement.Instance) error

// PlacementHelper helps the algorithm to place shards.
type PlacementHelper interface {
	// Instances returns the list of instances managed by the PlacementHelper.
	Instances() []placement.Instance

	// HasRackConflict checks if the rack constraint is violated when moving the shard to the target rack.
	HasRackConflict(shard uint32, from placement.Instance, toRack string) bool

	// PlaceShards distributes shards to the instances in the helper, with aware of where are the shards coming from.
	PlaceShards(shards []shard.Shard, from placement.Instance, candidates []placement.Instance) error

	// AddInstance adds an instance to the placement.
	AddInstance(addingInstance placement.Instance) error

	// Optimize rebalances the load distribution in the cluster.
	Optimize(t optimizeType) error

	// GeneratePlacement generates a placement.
	GeneratePlacement() placement.Placement

	// ReclaimLeavingShards reclaims all the leaving shards on the given instance
	// by pulling them back from the rest of the cluster.
	ReclaimLeavingShards(instance placement.Instance)

	// ReturnInitializingShards returns all the initializing shards on the given instance
	// by returning them back to the original owners.
	ReturnInitializingShards(instance placement.Instance)
}

type placementHelper struct {
	targetLoad         map[string]int
	shardToInstanceMap map[uint32]map[placement.Instance]struct{}
	rackToInstancesMap map[string]map[placement.Instance]struct{}
	rackToWeightMap    map[string]uint32
	totalWeight        uint32
	rf                 int
	uniqueShards       []uint32
	instances          map[string]placement.Instance
	log                log.Logger
	opts               placement.Options
}

// NewPlacementHelper returns a placement helper
func NewPlacementHelper(p placement.Placement, opts placement.Options) PlacementHelper {
	return newHelper(p, p.ReplicaFactor(), opts)
}

func newInitHelper(instances []placement.Instance, ids []uint32, opts placement.Options) PlacementHelper {
	emptyPlacement := placement.NewPlacement().
		SetInstances(instances).
		SetShards(ids).
		SetReplicaFactor(0).
		SetIsSharded(true).
		SetCutoverNanos(opts.PlacementCutoverNanosFn()())
	return newHelper(emptyPlacement, emptyPlacement.ReplicaFactor()+1, opts)
}

func newAddReplicaHelper(p placement.Placement, opts placement.Options) PlacementHelper {
	return newHelper(p, p.ReplicaFactor()+1, opts)
}

func newAddInstanceHelper(
	p placement.Placement,
	instance placement.Instance,
	opts placement.Options,
	t instanceType,
) (PlacementHelper, placement.Instance, error) {
	instanceInPlacement, exist := p.Instance(instance.ID())
	if !exist {
		return newHelper(p.SetInstances(append(p.Instances(), instance)), p.ReplicaFactor(), opts), instance, nil
	}

	switch t {
	case withLeavingShardsOnly:
		if !instanceInPlacement.IsLeaving() {
			return nil, nil, errInstanceContainsNonLeavingShards
		}
	case withAvailableOrLeavingShardsOnly:
		shards := instanceInPlacement.Shards()
		if shards.NumShards() != shards.NumShardsForState(shard.Available)+shards.NumShardsForState(shard.Leaving) {
			return nil, nil, errInstanceContainsInitializingShards
		}
	default:
		return nil, nil, fmt.Errorf("unexpected type %v", t)
	}

	return newHelper(p, p.ReplicaFactor(), opts), instanceInPlacement, nil
}

func newRemoveInstanceHelper(
	p placement.Placement,
	instanceID string,
	opts placement.Options,
) (PlacementHelper, placement.Instance, error) {
	p, leavingInstance, err := removeInstanceFromPlacement(p, instanceID)
	if err != nil {
		return nil, nil, err
	}
	return newHelper(p, p.ReplicaFactor(), opts), leavingInstance, nil
}

func newReplaceInstanceHelper(
	p placement.Placement,
	instanceIDs []string,
	addingInstances []placement.Instance,
	opts placement.Options,
) (PlacementHelper, []placement.Instance, []placement.Instance, error) {
	var (
		leavingInstances = make([]placement.Instance, len(instanceIDs))
		err              error
	)
	for i, instanceID := range instanceIDs {
		p, leavingInstances[i], err = removeInstanceFromPlacement(p, instanceID)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	newAddingInstances := make([]placement.Instance, len(addingInstances))
	for i, instance := range addingInstances {
		p, newAddingInstances[i], err = addInstanceToPlacement(p, instance, anyType)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return newHelper(p, p.ReplicaFactor(), opts), leavingInstances, newAddingInstances, nil
}

func newHelper(p placement.Placement, targetRF int, opts placement.Options) PlacementHelper {
	ph := &placementHelper{
		rf:           targetRF,
		instances:    make(map[string]placement.Instance, p.NumInstances()),
		uniqueShards: p.Shards(),
		log:          opts.InstrumentOptions().Logger(),
		opts:         opts,
	}

	for _, instance := range p.Instances() {
		ph.instances[instance.ID()] = instance
	}

	ph.scanCurrentLoad()
	ph.buildTargetLoad()
	return ph
}

func (ph *placementHelper) scanCurrentLoad() {
	ph.shardToInstanceMap = make(map[uint32]map[placement.Instance]struct{}, len(ph.uniqueShards))
	ph.rackToInstancesMap = make(map[string]map[placement.Instance]struct{})
	ph.rackToWeightMap = make(map[string]uint32)
	totalWeight := uint32(0)
	for _, instance := range ph.instances {
		if _, exist := ph.rackToInstancesMap[instance.Rack()]; !exist {
			ph.rackToInstancesMap[instance.Rack()] = make(map[placement.Instance]struct{})
		}
		ph.rackToInstancesMap[instance.Rack()][instance] = struct{}{}

		if instance.IsLeaving() {
			// Leaving instances are not counted as usable capacities in the placement.
			continue
		}

		ph.rackToWeightMap[instance.Rack()] = ph.rackToWeightMap[instance.Rack()] + instance.Weight()
		totalWeight += instance.Weight()

		for _, s := range instance.Shards().All() {
			if s.State() == shard.Leaving {
				continue
			}
			ph.assignShardToInstance(s, instance)
		}
	}
	ph.totalWeight = totalWeight
}

func (ph *placementHelper) buildTargetLoad() {
	overWeightedRack := 0
	overWeight := uint32(0)
	for _, weight := range ph.rackToWeightMap {
		if isRackOverWeight(weight, ph.totalWeight, ph.rf) {
			overWeightedRack++
			overWeight += weight
		}
	}

	targetLoad := make(map[string]int, len(ph.instances))
	for _, instance := range ph.instances {
		if instance.IsLeaving() {
			// We should not set a target load for leaving instances.
			continue
		}
		rackWeight := ph.rackToWeightMap[instance.Rack()]
		if isRackOverWeight(rackWeight, ph.totalWeight, ph.rf) {
			// if the instance is on a over-sized rack, the target load is topped at shardLen / rackSize
			targetLoad[instance.ID()] = int(math.Ceil(float64(ph.getShardLen()) * float64(instance.Weight()) / float64(rackWeight)))
		} else {
			// if the instance is on a normal rack, get the target load with aware of other over-sized rack
			targetLoad[instance.ID()] = ph.getShardLen() * (ph.rf - overWeightedRack) * int(instance.Weight()) / int(ph.totalWeight-overWeight)
		}
	}
	ph.targetLoad = targetLoad
}

func (ph *placementHelper) Instances() []placement.Instance {
	res := make([]placement.Instance, 0, len(ph.instances))
	for _, instance := range ph.instances {
		res = append(res, instance)
	}
	return res
}

func (ph *placementHelper) getShardLen() int {
	return len(ph.uniqueShards)
}

func (ph *placementHelper) targetLoadForInstance(id string) int {
	return ph.targetLoad[id]
}

func (ph *placementHelper) moveOneShard(from, to placement.Instance) bool {
	// The order matter here:
	// The Unknown shards were just moved, so free to be moved around.
	// The Initializing shards were still being initialized on the instance,
	// so moving them are cheaper than moving those Available shards.
	return ph.moveOneShardInState(from, to, shard.Unknown) ||
		ph.moveOneShardInState(from, to, shard.Initializing) ||
		ph.moveOneShardInState(from, to, shard.Available)
}

// nolint: unparam
func (ph *placementHelper) moveOneShardInState(from, to placement.Instance, state shard.State) bool {
	for _, s := range from.Shards().ShardsForState(state) {
		if ph.moveShard(s, from, to) {
			return true
		}
	}
	return false
}

func (ph *placementHelper) moveShard(candidateShard shard.Shard, from, to placement.Instance) bool {
	shardID := candidateShard.ID()
	if !ph.canAssignInstance(shardID, from, to) {
		return false
	}

	if candidateShard.State() == shard.Leaving {
		// should not move a Leaving shard,
		// Leaving shard will be removed when the Initializing shard is marked as Available
		return false
	}

	newShard := shard.NewShard(shardID)

	if from != nil {
		switch candidateShard.State() {
		case shard.Unknown, shard.Initializing:
			from.Shards().Remove(shardID)
			newShard.SetSourceID(candidateShard.SourceID())
		case shard.Available:
			candidateShard.
				SetState(shard.Leaving).
				SetCutoffNanos(ph.opts.ShardCutoffNanosFn()())
			newShard.SetSourceID(from.ID())
		}

		delete(ph.shardToInstanceMap[shardID], from)
	}

	curShard, ok := to.Shards().Shard(shardID)
	if ok && curShard.State() == shard.Leaving {
		// NB(cw): if the instance already owns the shard in Leaving state,
		// simply mark it as Available
		newShard = shard.NewShard(shardID).SetState(shard.Available)
		// NB(cw): Break the link between new owner of this shard with this Leaving instance
		instances := ph.shardToInstanceMap[shardID]
		for instance := range instances {
			shards := instance.Shards()
			initShard, ok := shards.Shard(shardID)
			if ok && initShard.SourceID() == to.ID() {
				initShard.SetSourceID("")
			}
		}

	}

	ph.assignShardToInstance(newShard, to)
	return true
}

func (ph *placementHelper) HasRackConflict(shard uint32, from placement.Instance, toRack string) bool {
	if from != nil {
		if from.Rack() == toRack {
			return false
		}
	}
	for instance := range ph.shardToInstanceMap[shard] {
		if instance.Rack() == toRack {
			return true
		}
	}
	return false
}

func (ph *placementHelper) buildInstanceHeap(instances []placement.Instance, availableCapacityAscending bool) (heap.Interface, error) {
	return newHeap(instances, availableCapacityAscending, ph.targetLoad, ph.rackToWeightMap)
}

func (ph *placementHelper) GeneratePlacement() placement.Placement {
	var instances = make([]placement.Instance, 0, len(ph.instances))

	for _, instance := range ph.instances {
		if instance.Shards().NumShards() > 0 {
			instances = append(instances, instance)
		}
	}

	for _, instance := range instances {
		shards := instance.Shards()
		for _, s := range shards.ShardsForState(shard.Unknown) {
			shards.Add(shard.NewShard(s.ID()).
				SetSourceID(s.SourceID()).
				SetState(shard.Initializing).
				SetCutoverNanos(ph.opts.ShardCutoverNanosFn()()))
		}
	}

	return placement.NewPlacement().
		SetInstances(instances).
		SetShards(ph.uniqueShards).
		SetReplicaFactor(ph.rf).
		SetIsSharded(true).
		SetIsMirrored(ph.opts.IsMirrored()).
		SetCutoverNanos(ph.opts.PlacementCutoverNanosFn()())
}

func (ph *placementHelper) PlaceShards(
	shards []shard.Shard,
	from placement.Instance,
	candidates []placement.Instance,
) error {
	shardSet := getShardMap(shards)
	if from != nil {
		// NB(cw) when removing an adding instance that has not finished bootstrapping its
		// Initializing shards, prefer to return those Initializing shards back to the leaving instance
		// to reduce some bootstrapping work in the cluster.
		ph.returnInitializingShardsToSource(shardSet, from, candidates)
	}

	instanceHeap, err := ph.buildInstanceHeap(nonLeavingInstances(candidates), true)
	if err != nil {
		return err
	}
	// if there are shards left to be assigned, distribute them evenly
	var triedInstances []placement.Instance
	for _, s := range shardSet {
		if s.State() == shard.Leaving {
			continue
		}
		moved := false
		for instanceHeap.Len() > 0 {
			tryInstance := heap.Pop(instanceHeap).(placement.Instance)
			triedInstances = append(triedInstances, tryInstance)
			if ph.moveShard(s, from, tryInstance) {
				moved = true
				break
			}
		}
		if !moved {
			// this should only happen when RF > number of racks
			return errNotEnoughRacks
		}
		for _, triedInstance := range triedInstances {
			heap.Push(instanceHeap, triedInstance)
		}
		triedInstances = triedInstances[:0]
	}
	return nil
}

func (ph *placementHelper) ReturnInitializingShards(instance placement.Instance) {
	shardSet := getShardMap(instance.Shards().All())
	ph.returnInitializingShardsToSource(shardSet, instance, ph.Instances())
}

func (ph *placementHelper) returnInitializingShardsToSource(
	shardSet map[uint32]shard.Shard,
	from placement.Instance,
	candidates []placement.Instance,
) {
	candidateMap := make(map[string]placement.Instance, len(candidates))
	for _, candidate := range candidates {
		candidateMap[candidate.ID()] = candidate
	}
	for _, s := range shardSet {
		if s.State() != shard.Initializing {
			continue
		}
		sourceID := s.SourceID()
		if sourceID == "" {
			continue
		}
		sourceInstance, ok := candidateMap[sourceID]
		if !ok {
			// NB(cw): This is not an error because the candidates are not
			// necessarily all the instances in the placement.
			continue
		}
		if sourceInstance.IsLeaving() {
			continue
		}
		if ph.moveShard(s, from, sourceInstance) {
			delete(shardSet, s.ID())
		}
	}
}

func (ph *placementHelper) mostUnderLoadedInstance() (placement.Instance, bool) {
	var (
		res        placement.Instance
		maxLoadGap int
	)

	for id, instance := range ph.instances {
		loadGap := ph.targetLoad[id] - loadOnInstance(instance)
		if loadGap > maxLoadGap {
			maxLoadGap = loadGap
			res = instance
		}
	}
	if maxLoadGap > 0 {
		return res, true
	}

	return nil, false
}

func (ph *placementHelper) Optimize(t optimizeType) error {
	var fn assignLoadFn
	switch t {
	case safe:
		fn = ph.assignLoadToInstanceSafe
	case unsafe:
		fn = ph.assignLoadToInstanceUnsafe
	}
	return ph.optimize(fn)
}

func (ph *placementHelper) optimize(fn assignLoadFn) error {
	uniq := make(map[string]struct{}, len(ph.instances))
	for {
		ins, ok := ph.mostUnderLoadedInstance()
		if !ok {
			return nil
		}
		if _, exist := uniq[ins.ID()]; exist {
			return nil
		}

		uniq[ins.ID()] = struct{}{}
		if err := fn(ins); err != nil {
			return err
		}
	}
}

func (ph *placementHelper) assignLoadToInstanceSafe(addingInstance placement.Instance) error {
	return ph.assignTargetLoad(addingInstance, func(from, to placement.Instance) bool {
		return ph.moveOneShardInState(from, to, shard.Unknown)
	})
}

func (ph *placementHelper) assignLoadToInstanceUnsafe(addingInstance placement.Instance) error {
	return ph.assignTargetLoad(addingInstance, func(from, to placement.Instance) bool {
		return ph.moveOneShard(from, to)
	})
}

func (ph *placementHelper) ReclaimLeavingShards(instance placement.Instance) {
	if instance.Shards().NumShardsForState(shard.Leaving) == 0 {
		// Shortcut if there is nothing to be reclaimed.
		return
	}
	id := instance.ID()
	for _, i := range ph.instances {
		for _, s := range i.Shards().ShardsForState(shard.Initializing) {
			if s.SourceID() == id {
				// NB(cw) in very rare case, the leaving shards could not be taken back.
				// For example: in a RF=2 case, instance a and b on rack1, instance c on rack2,
				// c took shard1 from instance a, before we tried to assign shard1 back to instance a,
				// b got assigned shard1, now if we try to add instance a back to the topology, a can
				// no longer take shard1 back.
				// But it's fine, the algo will fil up those load with other shards from the cluster
				ph.moveShard(s, i, instance)
			}
		}
	}
}

func (ph *placementHelper) AddInstance(addingInstance placement.Instance) error {
	ph.ReclaimLeavingShards(addingInstance)
	return ph.assignLoadToInstanceUnsafe(addingInstance)
}

func (ph *placementHelper) assignTargetLoad(
	targetInstance placement.Instance,
	moveOneShardFn func(from, to placement.Instance) bool,
) error {
	targetLoad := ph.targetLoadForInstance(targetInstance.ID())
	// try to take shards from the most loaded instances until the adding instance reaches target load
	instanceHeap, err := ph.buildInstanceHeap(nonLeavingInstances(ph.Instances()), false)
	if err != nil {
		return err
	}
	for targetInstance.Shards().NumShards() < targetLoad && instanceHeap.Len() > 0 {
		fromInstance := heap.Pop(instanceHeap).(placement.Instance)
		if moved := moveOneShardFn(fromInstance, targetInstance); moved {
			heap.Push(instanceHeap, fromInstance)
		}
	}
	return nil
}

func (ph *placementHelper) canAssignInstance(shardID uint32, from, to placement.Instance) bool {
	s, ok := to.Shards().Shard(shardID)
	if ok && s.State() != shard.Leaving {
		// NB(cw): a Leaving shard is not counted to the load of the instance
		// so the instance should be able to take the ownership back if needed
		// assuming i1 owns shard 1 as Available, this case can be triggered by:
		// 1: add i2, now shard 1 is "Leaving" on i1 and "Initializing" on i2
		// 2: remove i2, now i2 needs to return shard 1 back to i1
		// and i1 should be able to take it and mark it as "Available"
		return false
	}
	return ph.opts.LooseRackCheck() || !ph.HasRackConflict(shardID, from, to.Rack())
}

func (ph *placementHelper) assignShardToInstance(s shard.Shard, to placement.Instance) {
	to.Shards().Add(s)

	if _, exist := ph.shardToInstanceMap[s.ID()]; !exist {
		ph.shardToInstanceMap[s.ID()] = make(map[placement.Instance]struct{})
	}
	ph.shardToInstanceMap[s.ID()][to] = struct{}{}
}

// instanceHeap provides an easy way to get best candidate instance to assign/steal a shard
type instanceHeap struct {
	instances         []placement.Instance
	rackToWeightMap   map[string]uint32
	targetLoad        map[string]int
	capacityAscending bool
}

func newHeap(
	instances []placement.Instance,
	capacityAscending bool,
	targetLoad map[string]int,
	rackToWeightMap map[string]uint32,
) (*instanceHeap, error) {
	h := &instanceHeap{
		capacityAscending: capacityAscending,
		instances:         instances,
		targetLoad:        targetLoad,
		rackToWeightMap:   rackToWeightMap,
	}
	heap.Init(h)
	return h, nil
}

func (h *instanceHeap) targetLoadForInstance(id string) int {
	return h.targetLoad[id]
}

func (h *instanceHeap) Len() int {
	return len(h.instances)
}

func (h *instanceHeap) Less(i, j int) bool {
	instanceI := h.instances[i]
	instanceJ := h.instances[j]
	leftLoadOnI := h.targetLoadForInstance(instanceI.ID()) - loadOnInstance(instanceI)
	leftLoadOnJ := h.targetLoadForInstance(instanceJ.ID()) - loadOnInstance(instanceJ)
	// if both instance has tokens to be filled, prefer the one on a bigger rack
	// since it tends to be more picky in accepting shards
	if leftLoadOnI > 0 && leftLoadOnJ > 0 {
		if instanceI.Rack() != instanceJ.Rack() {
			return h.rackToWeightMap[instanceI.Rack()] > h.rackToWeightMap[instanceJ.Rack()]
		}
	}
	// compare left capacity on both instances
	if h.capacityAscending {
		return leftLoadOnI > leftLoadOnJ
	}
	return leftLoadOnI < leftLoadOnJ
}

func (h instanceHeap) Swap(i, j int) {
	h.instances[i], h.instances[j] = h.instances[j], h.instances[i]
}

func (h *instanceHeap) Push(i interface{}) {
	instance := i.(placement.Instance)
	h.instances = append(h.instances, instance)
}

func (h *instanceHeap) Pop() interface{} {
	n := len(h.instances)
	instance := h.instances[n-1]
	h.instances = h.instances[0 : n-1]
	return instance
}

func isRackOverWeight(rackWeight, totalWeight uint32, rf int) bool {
	return float64(rackWeight)/float64(totalWeight) >= 1.0/float64(rf)
}

func addInstanceToPlacement(
	p placement.Placement,
	i placement.Instance,
	t instanceType,
) (placement.Placement, placement.Instance, error) {
	if _, exist := p.Instance(i.ID()); exist {
		return nil, nil, errAddingInstanceAlreadyExist
	}

	switch t {
	case anyType:
	case withShards:
		if i.Shards().NumShards() == 0 {
			return p, i, nil
		}
	default:
		return nil, nil, fmt.Errorf("unexpected type %v", t)
	}

	instance := i.Clone()
	return p.SetInstances(append(p.Instances(), instance)), instance, nil
}

func removeInstanceFromPlacement(p placement.Placement, id string) (placement.Placement, placement.Instance, error) {
	leavingInstance, exist := p.Instance(id)
	if !exist {
		return nil, nil, fmt.Errorf("instance %s does not exist in placement", id)
	}
	return p.SetInstances(removeInstanceFromList(p.Instances(), id)), leavingInstance, nil
}

func getShardMap(shards []shard.Shard) map[uint32]shard.Shard {
	r := make(map[uint32]shard.Shard, len(shards))

	for _, s := range shards {
		r[s.ID()] = s
	}
	return r
}

func loadOnInstance(instance placement.Instance) int {
	return instance.Shards().NumShards() - instance.Shards().NumShardsForState(shard.Leaving)
}

func nonLeavingInstances(instances []placement.Instance) []placement.Instance {
	r := make([]placement.Instance, 0, len(instances))
	for _, instance := range instances {
		if instance.IsLeaving() {
			continue
		}
		r = append(r, instance)
	}

	return r
}

func newShards(shardIDs []uint32) []shard.Shard {
	r := make([]shard.Shard, len(shardIDs))
	for i, id := range shardIDs {
		r[i] = shard.NewShard(id).SetState(shard.Unknown)
	}
	return r
}

func removeInstanceFromList(instances []placement.Instance, instanceID string) []placement.Instance {
	for i, instance := range instances {
		if instance.ID() == instanceID {
			last := len(instances) - 1
			instances[i] = instances[last]
			return instances[:last]
		}
	}
	return instances
}

func markShardAvailable(p placement.Placement, instanceID string, shardID uint32, opts placement.Options) (placement.Placement, error) {
	instance, exist := p.Instance(instanceID)
	if !exist {
		return nil, fmt.Errorf("instance %s does not exist in placement", instanceID)
	}

	shards := instance.Shards()
	s, exist := shards.Shard(shardID)
	if !exist {
		return nil, fmt.Errorf("shard %d does not exist in instance %s", shardID, instanceID)
	}

	if s.State() != shard.Initializing {
		return nil, fmt.Errorf("could not mark shard %d as available, it's not in Initializing state", s.ID())
	}

	isCutoverFn := opts.IsShardCutoverFn()
	if isCutoverFn != nil {
		if err := isCutoverFn(s); err != nil {
			return nil, err
		}
	}

	sourceID := s.SourceID()
	shards.Add(shard.NewShard(shardID).SetState(shard.Available))

	// There could be no source for cases like initial placement.
	if sourceID == "" {
		return p, nil
	}

	sourceInstance, exist := p.Instance(sourceID)
	if !exist {
		return nil, fmt.Errorf("source instance %s for shard %d does not exist in placement", sourceID, shardID)
	}

	sourceShards := sourceInstance.Shards()
	leavingShard, exist := sourceShards.Shard(shardID)
	if !exist {
		return nil, fmt.Errorf("shard %d does not exist in source instance %s", shardID, sourceID)
	}

	if leavingShard.State() != shard.Leaving {
		return nil, fmt.Errorf("shard %d is not leaving instance %s", shardID, sourceID)
	}

	isCutoffFn := opts.IsShardCutoffFn()
	if isCutoffFn != nil {
		if err := isCutoffFn(leavingShard); err != nil {
			return nil, err
		}
	}

	sourceShards.Remove(shardID)
	if sourceShards.NumShards() == 0 {
		return p.SetInstances(removeInstanceFromList(p.Instances(), sourceInstance.ID())), nil
	}
	return p, nil
}

func markAllShardsAvailable(p placement.Placement, opts placement.Options) (placement.Placement, error) {
	p = p.Clone()
	var err error
	for _, instance := range p.Instances() {
		for _, s := range instance.Shards().All() {
			if s.State() == shard.Initializing {
				p, err = markShardAvailable(p, instance.ID(), s.ID(), opts)
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return p, nil
}
