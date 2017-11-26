// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package chunk

import (
	"fmt"
	"math"
	"testing"
	"time"

	. "github.com/pingcap/check"
	"github.com/pingcap/tidb/types"
	"github.com/pingcap/tidb/types/json"
)

func TestT(t *testing.T) {
	TestingT(t)
}

var _ = Suite(&testChunkSuite{})

type testChunkSuite struct{}

func (s *testChunkSuite) TestChunk(c *C) {
	numCols := 6
	numRows := 10
	chk := newChunk(8, 8, 0, 0, 40, -1)
	strFmt := "%d.12345"
	for i := 0; i < numRows; i++ {
		chk.AppendNull(0)
		chk.AppendInt64(1, int64(i))
		str := fmt.Sprintf(strFmt, i)
		chk.AppendString(2, str)
		chk.AppendBytes(3, []byte(str))
		chk.AppendMyDecimal(4, types.NewDecFromStringForTest(str))
		chk.AppendJSON(5, json.CreateJSON(str))
	}
	c.Assert(chk.NumCols(), Equals, numCols)
	c.Assert(chk.NumRows(), Equals, numRows)
	for i := 0; i < numRows; i++ {
		row := chk.GetRow(i)
		c.Assert(row.GetInt64(0), Equals, int64(0))
		c.Assert(row.IsNull(0), IsTrue)
		c.Assert(row.GetInt64(1), Equals, int64(i))
		str := fmt.Sprintf(strFmt, i)
		c.Assert(row.IsNull(2), IsFalse)
		c.Assert(row.GetString(2), Equals, str)
		c.Assert(row.IsNull(3), IsFalse)
		c.Assert(row.GetBytes(3), BytesEquals, []byte(str))
		c.Assert(row.IsNull(4), IsFalse)
		c.Assert(row.GetMyDecimal(4).String(), Equals, str)
		c.Assert(row.IsNull(5), IsFalse)
		c.Assert(row.GetJSON(5).Str, Equals, str)
	}

	chk2 := newChunk(8, 8, 0, 0, 40, -1)
	for i := 0; i < numRows; i++ {
		row := chk.GetRow(i)
		chk2.AppendRow(0, row)
	}
	for i := 0; i < numCols; i++ {
		col2, col1 := chk2.columns[i], chk.columns[i]
		col2.elemBuf, col1.elemBuf = nil, nil
		c.Assert(col2, DeepEquals, col1)
	}

	chk = newChunk(4, 8, -1, 16, 0, 0)
	f32Val := float32(1.2)
	chk.AppendFloat32(0, f32Val)
	f64Val := 1.3
	chk.AppendFloat64(1, f64Val)
	tVal := types.TimeFromDays(1)
	chk.AppendTime(2, tVal)
	durVal := types.Duration{Duration: time.Hour, Fsp: 6}
	chk.AppendDuration(3, durVal)
	enumVal := types.Enum{Name: "abc", Value: 100}
	chk.AppendEnum(4, enumVal)
	setVal := types.Set{Name: "def", Value: 101}
	chk.AppendSet(5, setVal)

	row := chk.GetRow(0)
	c.Assert(row.GetFloat32(0), Equals, f32Val)
	c.Assert(row.GetTime(2).Compare(tVal), Equals, 0)
	c.Assert(row.GetDuration(3), DeepEquals, durVal)
	c.Assert(row.GetEnum(4), DeepEquals, enumVal)
	c.Assert(row.GetSet(5), DeepEquals, setVal)

	// AppendRow can be different number of columns, useful for join.
	chk = newChunk(8, 8)
	chk2 = newChunk(8)
	chk2.AppendInt64(0, 1)
	chk2.AppendInt64(0, -1)
	chk.AppendRow(0, chk2.GetRow(0))
	chk.AppendRow(1, chk2.GetRow(0))
	c.Assert(chk.GetRow(0).GetInt64(0), Equals, int64(1))
	c.Assert(chk.GetRow(0).GetInt64(1), Equals, int64(1))
	c.Assert(chk.NumRows(), Equals, 1)

	// Test Reset.
	chk = newChunk(0)
	chk.AppendString(0, "abcd")
	chk.Reset()
	chk.AppendString(0, "def")
	c.Assert(chk.GetRow(0).GetString(0), Equals, "def")

	// Test float32
	chk = newChunk(4)
	chk.AppendFloat32(0, 1)
	chk.AppendFloat32(0, 1)
	chk.AppendFloat32(0, 1)
	c.Assert(chk.GetRow(2).GetFloat32(0), Equals, float32(1))
}

// newChunk creates a new chunk and initialize columns with element length.
// 0 adds an varlen column, positive len add a fixed length column, negative len adds a interface column.
func newChunk(elemLen ...int) *Chunk {
	chk := &Chunk{}
	for _, l := range elemLen {
		if l > 0 {
			chk.addFixedLenColumn(l, 0)
		} else if l == 0 {
			chk.addVarLenColumn(0)
		} else {
			chk.addInterfaceColumn()
		}
	}
	return chk
}

func BenchmarkAppendInt(b *testing.B) {
	b.ReportAllocs()
	chk := newChunk(8)
	for i := 0; i < b.N; i++ {
		appendInt(chk)
	}
}

func appendInt(chk *Chunk) {
	chk.Reset()
	for i := 0; i < 1000; i++ {
		chk.AppendInt64(0, int64(i))
	}
}

func BenchmarkAppendString(b *testing.B) {
	b.ReportAllocs()
	chk := newChunk(0)
	for i := 0; i < b.N; i++ {
		appendString(chk)
	}
}

func appendString(chk *Chunk) {
	chk.Reset()
	for i := 0; i < 1000; i++ {
		chk.AppendString(0, "abcd")
	}
}

func BenchmarkAppendRow(b *testing.B) {
	b.ReportAllocs()
	rowChk := newChunk(8, 8, 0, 0)
	rowChk.AppendNull(0)
	rowChk.AppendInt64(1, 1)
	rowChk.AppendString(2, "abcd")
	rowChk.AppendBytes(3, []byte("abcd"))

	chk := newChunk(8, 8, 0, 0)
	for i := 0; i < b.N; i++ {
		appendRow(chk, rowChk.GetRow(0))
	}
}

func appendRow(chk *Chunk, row Row) {
	chk.Reset()
	for i := 0; i < 1000; i++ {
		chk.AppendRow(0, row)
	}
}

func BenchmarkAccess(b *testing.B) {
	b.StopTimer()
	rowChk := newChunk(8)
	for i := 0; i < 8192; i++ {
		rowChk.AppendInt64(0, math.MaxUint16)
	}
	b.StartTimer()
	var sum int64
	for i := 0; i < b.N; i++ {
		for j := 0; j < 8192; j++ {
			sum += rowChk.GetRow(j).GetInt64(0)
		}
	}
	fmt.Println(sum)
}