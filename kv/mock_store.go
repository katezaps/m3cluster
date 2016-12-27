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

// Automatically generated by MockGen. DO NOT EDIT!
// Source: github.com/m3db/m3cluster/kv/types.go

package kv

import (
	gomock "github.com/golang/mock/gomock"
	proto "github.com/golang/protobuf/proto"
)

// Mock of Value interface
type MockValue struct {
	ctrl     *gomock.Controller
	recorder *_MockValueRecorder
}

// Recorder for MockValue (not exported)
type _MockValueRecorder struct {
	mock *MockValue
}

func NewMockValue(ctrl *gomock.Controller) *MockValue {
	mock := &MockValue{ctrl: ctrl}
	mock.recorder = &_MockValueRecorder{mock}
	return mock
}

func (_m *MockValue) EXPECT() *_MockValueRecorder {
	return _m.recorder
}

func (_m *MockValue) Unmarshal(v proto.Message) error {
	ret := _m.ctrl.Call(_m, "Unmarshal", v)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockValueRecorder) Unmarshal(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Unmarshal", arg0)
}

func (_m *MockValue) Version() int {
	ret := _m.ctrl.Call(_m, "Version")
	ret0, _ := ret[0].(int)
	return ret0
}

func (_mr *_MockValueRecorder) Version() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Version")
}

// Mock of ValueWatch interface
type MockValueWatch struct {
	ctrl     *gomock.Controller
	recorder *_MockValueWatchRecorder
}

// Recorder for MockValueWatch (not exported)
type _MockValueWatchRecorder struct {
	mock *MockValueWatch
}

func NewMockValueWatch(ctrl *gomock.Controller) *MockValueWatch {
	mock := &MockValueWatch{ctrl: ctrl}
	mock.recorder = &_MockValueWatchRecorder{mock}
	return mock
}

func (_m *MockValueWatch) EXPECT() *_MockValueWatchRecorder {
	return _m.recorder
}

func (_m *MockValueWatch) C() <-chan struct{} {
	ret := _m.ctrl.Call(_m, "C")
	ret0, _ := ret[0].(<-chan struct{})
	return ret0
}

func (_mr *_MockValueWatchRecorder) C() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "C")
}

func (_m *MockValueWatch) Get() Value {
	ret := _m.ctrl.Call(_m, "Get")
	ret0, _ := ret[0].(Value)
	return ret0
}

func (_mr *_MockValueWatchRecorder) Get() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Get")
}

func (_m *MockValueWatch) Close() {
	_m.ctrl.Call(_m, "Close")
}

func (_mr *_MockValueWatchRecorder) Close() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Close")
}

// Mock of ValueWatchable interface
type MockValueWatchable struct {
	ctrl     *gomock.Controller
	recorder *_MockValueWatchableRecorder
}

// Recorder for MockValueWatchable (not exported)
type _MockValueWatchableRecorder struct {
	mock *MockValueWatchable
}

func NewMockValueWatchable(ctrl *gomock.Controller) *MockValueWatchable {
	mock := &MockValueWatchable{ctrl: ctrl}
	mock.recorder = &_MockValueWatchableRecorder{mock}
	return mock
}

func (_m *MockValueWatchable) EXPECT() *_MockValueWatchableRecorder {
	return _m.recorder
}

func (_m *MockValueWatchable) Get() Value {
	ret := _m.ctrl.Call(_m, "Get")
	ret0, _ := ret[0].(Value)
	return ret0
}

func (_mr *_MockValueWatchableRecorder) Get() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Get")
}

func (_m *MockValueWatchable) Watch() (Value, ValueWatch, error) {
	ret := _m.ctrl.Call(_m, "Watch")
	ret0, _ := ret[0].(Value)
	ret1, _ := ret[1].(ValueWatch)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

func (_mr *_MockValueWatchableRecorder) Watch() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Watch")
}

func (_m *MockValueWatchable) NumWatches() int {
	ret := _m.ctrl.Call(_m, "NumWatches")
	ret0, _ := ret[0].(int)
	return ret0
}

func (_mr *_MockValueWatchableRecorder) NumWatches() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "NumWatches")
}

func (_m *MockValueWatchable) Update(_param0 Value) error {
	ret := _m.ctrl.Call(_m, "Update", _param0)
	ret0, _ := ret[0].(error)
	return ret0
}

func (_mr *_MockValueWatchableRecorder) Update(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Update", arg0)
}

func (_m *MockValueWatchable) IsClosed() bool {
	ret := _m.ctrl.Call(_m, "IsClosed")
	ret0, _ := ret[0].(bool)
	return ret0
}

func (_mr *_MockValueWatchableRecorder) IsClosed() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "IsClosed")
}

func (_m *MockValueWatchable) Close() {
	_m.ctrl.Call(_m, "Close")
}

func (_mr *_MockValueWatchableRecorder) Close() *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Close")
}

// Mock of Store interface
type MockStore struct {
	ctrl     *gomock.Controller
	recorder *_MockStoreRecorder
}

// Recorder for MockStore (not exported)
type _MockStoreRecorder struct {
	mock *MockStore
}

func NewMockStore(ctrl *gomock.Controller) *MockStore {
	mock := &MockStore{ctrl: ctrl}
	mock.recorder = &_MockStoreRecorder{mock}
	return mock
}

func (_m *MockStore) EXPECT() *_MockStoreRecorder {
	return _m.recorder
}

func (_m *MockStore) Get(key string) (Value, error) {
	ret := _m.ctrl.Call(_m, "Get", key)
	ret0, _ := ret[0].(Value)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockStoreRecorder) Get(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Get", arg0)
}

func (_m *MockStore) Watch(key string) (ValueWatch, error) {
	ret := _m.ctrl.Call(_m, "Watch", key)
	ret0, _ := ret[0].(ValueWatch)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockStoreRecorder) Watch(arg0 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Watch", arg0)
}

func (_m *MockStore) Set(key string, v proto.Message) (int, error) {
	ret := _m.ctrl.Call(_m, "Set", key, v)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockStoreRecorder) Set(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "Set", arg0, arg1)
}

func (_m *MockStore) SetIfNotExists(key string, v proto.Message) (int, error) {
	ret := _m.ctrl.Call(_m, "SetIfNotExists", key, v)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockStoreRecorder) SetIfNotExists(arg0, arg1 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "SetIfNotExists", arg0, arg1)
}

func (_m *MockStore) CheckAndSet(key string, version int, v proto.Message) (int, error) {
	ret := _m.ctrl.Call(_m, "CheckAndSet", key, version, v)
	ret0, _ := ret[0].(int)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

func (_mr *_MockStoreRecorder) CheckAndSet(arg0, arg1, arg2 interface{}) *gomock.Call {
	return _mr.mock.ctrl.RecordCall(_mr.mock, "CheckAndSet", arg0, arg1, arg2)
}
