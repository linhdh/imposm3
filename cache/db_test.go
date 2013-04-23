package cache

import (
	"goposm/element"
	"io/ioutil"
	"os"
	"testing"
)

func TestCreateCache(t *testing.T) {
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewCoordsCache(cache_dir)
	if err != nil {
		t.Fatal()
	}
	defer cache.Close()

	if stat, err := os.Stat(cache_dir); err != nil || !stat.IsDir() {
		t.Error("cache dir not created")
	}
}

func TestReadWriteCoord(t *testing.T) {
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewCoordsCache(cache_dir)
	if err != nil {
		t.Fatal()
	}
	node := &element.Node{}
	node.Id = 1
	cache.PutCoord(node)
	cache.Close()

	cache, err = NewCoordsCache(cache_dir)
	if err != nil {
		t.Fatal()
	}
	defer cache.Close()

	data, _ := cache.GetCoord(1)

	if data.Id != 1 {
		t.Errorf("unexpected result of GetNode(1): %v", data)
	}
}

func TestReadWriteNode(t *testing.T) {
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewNodesCache(cache_dir)
	if err != nil {
		t.Fatal()
	}
	node := &element.Node{
		OSMElem: element.OSMElem{
			Id:   1234,
			Tags: element.Tags{"foo": "bar"}},
	}
	cache.PutNode(node)
	cache.Close()

	cache, err = NewNodesCache(cache_dir)
	if err != nil {
		t.Fatal()
	}
	defer cache.Close()

	data, err := cache.GetNode(1234)
	if data.Id != 1234 || data.Tags["foo"] != "bar" {
		t.Errorf("unexpected result of GetNode: %v", data)
	}

	data, err = cache.GetNode(99)
	if data != nil {
		t.Error("missing node not nil")
	}

}

func TestReadWriteWay(t *testing.T) {
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewWaysCache(cache_dir)
	if err != nil {
		t.Fatal()
	}
	way := &element.Way{
		OSMElem: element.OSMElem{
			Id:   1234,
			Tags: element.Tags{"foo": "bar"}},
		Refs: []int64{942374923, 23948234},
	}
	cache.PutWay(way)
	cache.Close()

	cache, err = NewWaysCache(cache_dir)
	if err != nil {
		t.Fatal()
	}
	defer cache.Close()

	data, _ := cache.GetWay(1234)

	if data.Id != 1234 || data.Tags["foo"] != "bar" {
		t.Errorf("unexpected result of GetWay: %#v", data)
	}
	if len(data.Refs) != 2 ||
		data.Refs[0] != 942374923 ||
		data.Refs[1] != 23948234 {
		t.Errorf("unexpected result of GetWay: %#v", data)
	}
}

func TestReadMissingWay(t *testing.T) {
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewWaysCache(cache_dir)
	if err != nil {
		t.Fatal()
	}
	defer cache.Close()

	data, _ := cache.GetWay(1234)

	if data != nil {
		t.Errorf("missing way did not return nil")
	}
}

func BenchmarkWriteWay(b *testing.B) {
	b.StopTimer()
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewWaysCache(cache_dir)
	if err != nil {
		b.Fatal()
	}
	defer cache.Close()

	b.StartTimer()
	way := &element.Way{
		OSMElem: element.OSMElem{Tags: element.Tags{"foo": "bar"}},
		Refs:    []int64{942374923, 23948234},
	}
	for i := 0; i < b.N; i++ {
		way.Id = int64(i)
		cache.PutWay(way)
	}
}

func BenchmarkReadWay(b *testing.B) {
	b.StopTimer()
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewWaysCache(cache_dir)
	if err != nil {
		b.Fatal()
	}
	defer cache.Close()

	way := &element.Way{}
	for i := 0; i < b.N; i++ {
		way.Id = int64(i)
		cache.PutWay(way)
	}

	b.StartTimer()
	for i := int64(0); i < int64(b.N); i++ {
		if coord, err := cache.GetWay(i); err != nil || coord.Id != i {
			b.Fail()
		}
	}

}

func BenchmarkWriteCoord(b *testing.B) {
	b.StopTimer()
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewCoordsCache(cache_dir)
	if err != nil {
		b.Fatal()
	}
	defer cache.Close()

	b.StartTimer()
	node := &element.Node{}
	for i := 0; i < b.N; i++ {
		node.Id = int64(i)
		cache.PutCoord(node)
	}
}

func BenchmarkReadCoord(b *testing.B) {
	b.StopTimer()
	cache_dir, _ := ioutil.TempDir("", "goposm_test")
	defer os.RemoveAll(cache_dir)

	cache, err := NewCoordsCache(cache_dir)
	if err != nil {
		b.Fatal()
	}
	defer cache.Close()

	node := &element.Node{}
	for i := 0; i < b.N; i++ {
		node.Id = int64(i)
		cache.PutCoord(node)
	}

	b.StartTimer()
	for i := int64(0); i < int64(b.N); i++ {
		if coord, err := cache.GetCoord(i); err == nil || coord.Id != i {
			b.Fail()
		}
	}

}
