// Copyright 2020 ConsenSys Software Inc.
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

// Code generated by gurvy DO NOT EDIT

package bn256

import (
	"encoding/binary"
	"errors"
	"io"
	"math/big"

	"github.com/consensys/gurvy/bn256/fp"
	"github.com/consensys/gurvy/bn256/fr"
	"github.com/consensys/gurvy/utils"
	"github.com/consensys/gurvy/utils/debug"
	"github.com/consensys/gurvy/utils/parallel"
)

// G1 point in affine coordinates
type G1 struct {
	X, Y fp.Element
}

// g1Jac is a point with fp.Element coordinates
type g1Jac struct {
	X, Y, Z fp.Element
}

//  g1JacExtended parameterized jacobian coordinates (x=X/ZZ, y=Y/ZZZ, ZZ**3=ZZZ**2)
type g1JacExtended struct {
	X, Y, ZZ, ZZZ fp.Element
}

// g1Proj point in projective coordinates
type g1Proj struct {
	x, y, z fp.Element
}

// -------------------------------------------------------------------------------------------------
// Affine

// ScalarMultiplication computes and returns p = a*s
func (p *G1) ScalarMultiplication(a *G1, s *big.Int) *G1 {
	var _p g1Jac
	_p.FromAffine(a)
	_p.mulGLV(&_p, s)
	p.FromJacobian(&_p)
	return p
}

// Equal tests if two points (in Affine coordinates) are equal
func (p *G1) Equal(a *G1) bool {
	return p.X.Equal(&a.X) && p.Y.Equal(&a.Y)
}

// Neg computes -G
func (p *G1) Neg(a *G1) *G1 {
	p.X = a.X
	p.Y.Neg(&a.Y)
	return p
}

// FromJacobian rescale a point in Jacobian coord in z=1 plane
func (p *G1) FromJacobian(p1 *g1Jac) *G1 {

	var a, b fp.Element

	if p1.Z.IsZero() {
		p.X.SetZero()
		p.Y.SetZero()
		return p
	}

	a.Inverse(&p1.Z)
	b.Square(&a)
	p.X.Mul(&p1.X, &b)
	p.Y.Mul(&p1.Y, &b).Mul(&p.Y, &a)

	return p
}

func (p *G1) String() string {
	var x, y fp.Element
	x.Set(&p.X)
	y.Set(&p.Y)
	return "E([" + x.String() + "," + y.String() + "]),"
}

// IsInfinity checks if the point is infinity (in affine, it's encoded as (0,0))
func (p *G1) IsInfinity() bool {
	return p.X.IsZero() && p.Y.IsZero()
}

// IsOnCurve returns true if p in on the curve
func (p *G1) IsOnCurve() bool {
	var point g1Jac
	point.FromAffine(p)
	return point.IsOnCurve() // call this function to handle infinity point
}

// IsInSubGroup returns true if p is in the correct subgroup, false otherwise
func (p *G1) IsInSubGroup() bool {
	var _p g1Jac
	_p.FromAffine(p)
	return _p.IsOnCurve() && _p.IsInSubGroup()
}

// -------------------------------------------------------------------------------------------------
// Jacobian

// Set set p to the provided point
func (p *g1Jac) Set(a *g1Jac) *g1Jac {
	p.X, p.Y, p.Z = a.X, a.Y, a.Z
	return p
}

// Equal tests if two points (in Jacobian coordinates) are equal
func (p *g1Jac) Equal(a *g1Jac) bool {

	if p.Z.IsZero() && a.Z.IsZero() {
		return true
	}
	_p := G1{}
	_p.FromJacobian(p)

	_a := G1{}
	_a.FromJacobian(a)

	return _p.X.Equal(&_a.X) && _p.Y.Equal(&_a.Y)
}

// Neg computes -G
func (p *g1Jac) Neg(a *g1Jac) *g1Jac {
	*p = *a
	p.Y.Neg(&a.Y)
	return p
}

// SubAssign substracts two points on the curve
func (p *g1Jac) SubAssign(a *g1Jac) *g1Jac {
	var tmp g1Jac
	tmp.Set(a)
	tmp.Y.Neg(&tmp.Y)
	p.AddAssign(&tmp)
	return p
}

// AddAssign point addition in montgomery form
// https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-3.html#addition-add-2007-bl
func (p *g1Jac) AddAssign(a *g1Jac) *g1Jac {

	// p is infinity, return a
	if p.Z.IsZero() {
		p.Set(a)
		return p
	}

	// a is infinity, return p
	if a.Z.IsZero() {
		return p
	}

	var Z1Z1, Z2Z2, U1, U2, S1, S2, H, I, J, r, V fp.Element
	Z1Z1.Square(&a.Z)
	Z2Z2.Square(&p.Z)
	U1.Mul(&a.X, &Z2Z2)
	U2.Mul(&p.X, &Z1Z1)
	S1.Mul(&a.Y, &p.Z).
		Mul(&S1, &Z2Z2)
	S2.Mul(&p.Y, &a.Z).
		Mul(&S2, &Z1Z1)

	// if p == a, we double instead
	if U1.Equal(&U2) && S1.Equal(&S2) {
		return p.DoubleAssign()
	}

	H.Sub(&U2, &U1)
	I.Double(&H).
		Square(&I)
	J.Mul(&H, &I)
	r.Sub(&S2, &S1).Double(&r)
	V.Mul(&U1, &I)
	p.X.Square(&r).
		Sub(&p.X, &J).
		Sub(&p.X, &V).
		Sub(&p.X, &V)
	p.Y.Sub(&V, &p.X).
		Mul(&p.Y, &r)
	S1.Mul(&S1, &J).Double(&S1)
	p.Y.Sub(&p.Y, &S1)
	p.Z.Add(&p.Z, &a.Z)
	p.Z.Square(&p.Z).
		Sub(&p.Z, &Z1Z1).
		Sub(&p.Z, &Z2Z2).
		Mul(&p.Z, &H)

	return p
}

// AddMixed point addition
// http://www.hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-0.html#addition-madd-2007-bl
func (p *g1Jac) AddMixed(a *G1) *g1Jac {

	//if a is infinity return p
	if a.X.IsZero() && a.Y.IsZero() {
		return p
	}
	// p is infinity, return a
	if p.Z.IsZero() {
		p.X = a.X
		p.Y = a.Y
		p.Z.SetOne()
		return p
	}

	// get some Element from our pool
	var Z1Z1, U2, S2, H, HH, I, J, r, V fp.Element
	Z1Z1.Square(&p.Z)
	U2.Mul(&a.X, &Z1Z1)
	S2.Mul(&a.Y, &p.Z).
		Mul(&S2, &Z1Z1)

	// if p == a, we double instead
	if U2.Equal(&p.X) && S2.Equal(&p.Y) {
		return p.DoubleAssign()
	}

	H.Sub(&U2, &p.X)
	HH.Square(&H)
	I.Double(&HH).Double(&I)
	J.Mul(&H, &I)
	r.Sub(&S2, &p.Y).Double(&r)
	V.Mul(&p.X, &I)
	p.X.Square(&r).
		Sub(&p.X, &J).
		Sub(&p.X, &V).
		Sub(&p.X, &V)
	J.Mul(&J, &p.Y).Double(&J)
	p.Y.Sub(&V, &p.X).
		Mul(&p.Y, &r)
	p.Y.Sub(&p.Y, &J)
	p.Z.Add(&p.Z, &H)
	p.Z.Square(&p.Z).
		Sub(&p.Z, &Z1Z1).
		Sub(&p.Z, &HH)

	return p
}

// Double doubles a point in Jacobian coordinates
// https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-3.html#doubling-dbl-2007-bl
func (p *g1Jac) Double(q *g1Jac) *g1Jac {
	p.Set(q)
	p.DoubleAssign()
	return p
}

// DoubleAssign doubles a point in Jacobian coordinates
// https://hyperelliptic.org/EFD/g1p/auto-shortw-jacobian-3.html#doubling-dbl-2007-bl
func (p *g1Jac) DoubleAssign() *g1Jac {

	// get some Element from our pool
	var XX, YY, YYYY, ZZ, S, M, T fp.Element

	XX.Square(&p.X)
	YY.Square(&p.Y)
	YYYY.Square(&YY)
	ZZ.Square(&p.Z)
	S.Add(&p.X, &YY)
	S.Square(&S).
		Sub(&S, &XX).
		Sub(&S, &YYYY).
		Double(&S)
	M.Double(&XX).Add(&M, &XX)
	p.Z.Add(&p.Z, &p.Y).
		Square(&p.Z).
		Sub(&p.Z, &YY).
		Sub(&p.Z, &ZZ)
	T.Square(&M)
	p.X = T
	T.Double(&S)
	p.X.Sub(&p.X, &T)
	p.Y.Sub(&S, &p.X).
		Mul(&p.Y, &M)
	YYYY.Double(&YYYY).Double(&YYYY).Double(&YYYY)
	p.Y.Sub(&p.Y, &YYYY)

	return p
}

// ScalarMultiplication computes and returns p = a*s
// see https://www.iacr.org/archive/crypto2001/21390189.pdf
func (p *g1Jac) ScalarMultiplication(a *g1Jac, s *big.Int) *g1Jac {
	return p.mulGLV(a, s)
}

func (p *g1Jac) String() string {
	if p.Z.IsZero() {
		return "O"
	}
	_p := G1{}
	_p.FromJacobian(p)
	return "E([" + _p.X.String() + "," + _p.Y.String() + "]),"
}

// FromAffine sets p = Q, p in Jacboian, Q in affine
func (p *g1Jac) FromAffine(Q *G1) *g1Jac {
	if Q.X.IsZero() && Q.Y.IsZero() {
		p.Z.SetZero()
		p.X.SetOne()
		p.Y.SetOne()
		return p
	}
	p.Z.SetOne()
	p.X.Set(&Q.X)
	p.Y.Set(&Q.Y)
	return p
}

// IsOnCurve returns true if p in on the curve
func (p *g1Jac) IsOnCurve() bool {
	var left, right, tmp fp.Element
	left.Square(&p.Y)
	right.Square(&p.X).Mul(&right, &p.X)
	tmp.Square(&p.Z).
		Square(&tmp).
		Mul(&tmp, &p.Z).
		Mul(&tmp, &p.Z).
		Mul(&tmp, &bCurveCoeff)
	right.Add(&right, &tmp)
	return left.Equal(&right)
}

// IsInSubGroup returns true if p is on the r-torsion, false otherwise.
// For bn curves, the r-torsion in E(Fp) is the full group, so we just check that
// the point is on the curve.
func (p *g1Jac) IsInSubGroup() bool {

	return p.IsOnCurve()

}

// mulWindowed 2-bits windowed exponentiation
func (p *g1Jac) mulWindowed(a *g1Jac, s *big.Int) *g1Jac {

	var res g1Jac
	var ops [3]g1Jac

	res.Set(&g1Infinity)
	ops[0].Set(a)
	ops[1].Double(&ops[0])
	ops[2].Set(&ops[0]).AddAssign(&ops[1])

	b := s.Bytes()
	for i := range b {
		w := b[i]
		mask := byte(0xc0)
		for j := 0; j < 4; j++ {
			res.DoubleAssign().DoubleAssign()
			c := (w & mask) >> (6 - 2*j)
			if c != 0 {
				res.AddAssign(&ops[c-1])
			}
			mask = mask >> 2
		}
	}
	p.Set(&res)

	return p

}

// phi assigns p to phi(a) where phi: (x,y)->(ux,y), and returns p
func (p *g1Jac) phi(a *g1Jac) *g1Jac {
	p.Set(a)

	p.X.Mul(&p.X, &thirdRootOneG1)

	return p
}

// mulGLV performs scalar multiplication using GLV
// see https://www.iacr.org/archive/crypto2001/21390189.pdf
func (p *g1Jac) mulGLV(a *g1Jac, s *big.Int) *g1Jac {

	var table [15]g1Jac
	var zero big.Int
	var res g1Jac
	var k1, k2 fr.Element

	res.Set(&g1Infinity)

	// table[b3b2b1b0-1] = b3b2*phi(a) + b1b0*a
	table[0].Set(a)
	table[3].phi(a)

	// split the scalar, modifies +-a, phi(a) accordingly
	k := utils.SplitScalar(s, &glvBasis)

	if k[0].Cmp(&zero) == -1 {
		k[0].Neg(&k[0])
		table[0].Neg(&table[0])
	}
	if k[1].Cmp(&zero) == -1 {
		k[1].Neg(&k[1])
		table[3].Neg(&table[3])
	}

	// precompute table (2 bits sliding window)
	// table[b3b2b1b0-1] = b3b2*phi(a) + b1b0*a if b3b2b1b0 != 0
	table[1].Double(&table[0])
	table[2].Set(&table[1]).AddAssign(&table[0])
	table[4].Set(&table[3]).AddAssign(&table[0])
	table[5].Set(&table[3]).AddAssign(&table[1])
	table[6].Set(&table[3]).AddAssign(&table[2])
	table[7].Double(&table[3])
	table[8].Set(&table[7]).AddAssign(&table[0])
	table[9].Set(&table[7]).AddAssign(&table[1])
	table[10].Set(&table[7]).AddAssign(&table[2])
	table[11].Set(&table[7]).AddAssign(&table[3])
	table[12].Set(&table[11]).AddAssign(&table[0])
	table[13].Set(&table[11]).AddAssign(&table[1])
	table[14].Set(&table[11]).AddAssign(&table[2])

	// bounds on the lattice base vectors guarantee that k1, k2 are len(r)/2 bits long max
	k1.SetBigInt(&k[0]).FromMont()
	k2.SetBigInt(&k[1]).FromMont()

	// loop starts from len(k1)/2 due to the bounds
	for i := len(k1)/2 - 1; i >= 0; i-- {
		mask := uint64(3) << 62
		for j := 0; j < 32; j++ {
			res.Double(&res).Double(&res)
			b1 := (k1[i] & mask) >> (62 - 2*j)
			b2 := (k2[i] & mask) >> (62 - 2*j)
			if b1|b2 != 0 {
				s := (b2<<2 | b1)
				res.AddAssign(&table[s-1])
			}
			mask = mask >> 2
		}
	}

	p.Set(&res)
	return p
}

// -------------------------------------------------------------------------------------------------
// Jacobian extended

// -------------------------------------------------------------------------------------------------
// Projective

// FromJacobian converts a point from Jacobian to projective coordinates
func (p *g1Proj) FromJacobian(Q *g1Jac) *g1Proj {
	// memalloc
	var buf fp.Element
	buf.Square(&Q.Z)

	p.x.Mul(&Q.X, &Q.Z)
	p.y.Set(&Q.Y)
	p.z.Mul(&Q.Z, &buf)

	return p
}

// BatchJacobianToAffineG1 converts points in Jacobian coordinates to Affine coordinates
// performing a single field inversion (Montgomery batch inversion trick)
// result must be allocated with len(result) == len(points)
func BatchJacobianToAffineG1(points []g1Jac, result []G1) {
	debug.Assert(len(result) == len(points))
	zeroes := make([]bool, len(points))
	accumulator := fp.One()

	// batch invert all points[].Z coordinates with Montgomery batch inversion trick
	// (stores points[].Z^-1 in result[i].X to avoid allocating a slice of fr.Elements)
	for i := 0; i < len(points); i++ {
		if points[i].Z.IsZero() {
			zeroes[i] = true
			continue
		}
		result[i].X = accumulator
		accumulator.Mul(&accumulator, &points[i].Z)
	}

	var accInverse fp.Element
	accInverse.Inverse(&accumulator)

	for i := len(points) - 1; i >= 0; i-- {
		if zeroes[i] {
			// do nothing, X and Y are zeroes in affine.
			continue
		}
		result[i].X.Mul(&result[i].X, &accInverse)
		accInverse.Mul(&accInverse, &points[i].Z)
	}

	// batch convert to affine.
	parallel.Execute(len(points), func(start, end int) {
		for i := start; i < end; i++ {
			if zeroes[i] {
				// do nothing, X and Y are zeroes in affine.
				continue
			}
			var a, b fp.Element
			a = result[i].X
			b.Square(&a)
			result[i].X.Mul(&points[i].X, &b)
			result[i].Y.Mul(&points[i].Y, &b).
				Mul(&result[i].Y, &a)
		}
	})

}

// BatchScalarMultiplicationG1 multiplies the same base (generator) by all scalars
// and return resulting points in affine coordinates
// uses a simple windowed-NAF like exponentiation algorithm
func BatchScalarMultiplicationG1(base *G1, scalars []fr.Element) []G1 {

	// approximate cost in group ops is
	// cost = 2^{c-1} + n(scalar.nbBits+nbChunks)

	nbPoints := uint64(len(scalars))
	min := ^uint64(0)
	bestC := 0
	for c := 2; c < 18; c++ {
		cost := uint64(1 << (c - 1))
		nbChunks := uint64(fr.Limbs * 64 / c)
		if (fr.Limbs*64)%c != 0 {
			nbChunks++
		}
		cost += nbPoints * ((fr.Limbs * 64) + nbChunks)
		if cost < min {
			min = cost
			bestC = c
		}
	}
	c := uint64(bestC) // window size
	nbChunks := int(fr.Limbs * 64 / c)
	if (fr.Limbs*64)%c != 0 {
		nbChunks++
	}
	mask := uint64((1 << c) - 1) // low c bits are 1
	msbWindow := uint64(1 << (c - 1))

	// precompute all powers of base for our window
	// note here that if performance is critical, we can implement as in the msmX methods
	// this allocation to be on the stack
	baseTable := make([]g1Jac, (1 << (c - 1)))
	baseTable[0].Set(&g1Infinity)
	baseTable[0].AddMixed(base)
	for i := 1; i < len(baseTable); i++ {
		baseTable[i] = baseTable[i-1]
		baseTable[i].AddMixed(base)
	}

	pScalars := partitionScalars(scalars, c)

	// compute offset and word selector / shift to select the right bits of our windows
	selectors := make([]selector, nbChunks)
	for chunk := 0; chunk < nbChunks; chunk++ {
		jc := uint64(uint64(chunk) * c)
		d := selector{}
		d.index = jc / 64
		d.shift = jc - (d.index * 64)
		d.mask = mask << d.shift
		d.multiWordSelect = (64%c) != 0 && d.shift > (64-c) && d.index < (fr.Limbs-1)
		if d.multiWordSelect {
			nbBitsHigh := d.shift - uint64(64-c)
			d.maskHigh = (1 << nbBitsHigh) - 1
			d.shiftHigh = (c - nbBitsHigh)
		}
		selectors[chunk] = d
	}

	// convert our base exp table into affine to use AddMixed
	baseTableAff := make([]G1, (1 << (c - 1)))
	BatchJacobianToAffineG1(baseTable, baseTableAff)
	toReturn := make([]g1Jac, len(scalars))

	// for each digit, take value in the base table, double it c time, voila.
	parallel.Execute(len(pScalars), func(start, end int) {
		var p g1Jac
		for i := start; i < end; i++ {
			p.Set(&g1Infinity)
			for chunk := nbChunks - 1; chunk >= 0; chunk-- {
				s := selectors[chunk]
				if chunk != nbChunks-1 {
					for j := uint64(0); j < c; j++ {
						p.DoubleAssign()
					}
				}

				bits := (pScalars[i][s.index] & s.mask) >> s.shift
				if s.multiWordSelect {
					bits += (pScalars[i][s.index+1] & s.maskHigh) << s.shiftHigh
				}

				if bits == 0 {
					continue
				}

				// if msbWindow bit is set, we need to substract
				if bits&msbWindow == 0 {
					// add

					p.AddMixed(&baseTableAff[bits-1])

				} else {
					// sub

					t := baseTableAff[bits & ^msbWindow]
					t.Neg(&t)
					p.AddMixed(&t)

				}
			}

			// set our result point

			toReturn[i] = p

		}
	})

	toReturnAff := make([]G1, len(scalars))
	BatchJacobianToAffineG1(toReturn, toReturnAff)
	return toReturnAff

}

// SizeOfG1Compressed represents the size in bytes that a G1 need in binary form, compressed
const SizeOfG1Compressed = 32

// SizeOfG1Uncompressed represents the size in bytes that a G1 need in binary form, uncompressed
const SizeOfG1Uncompressed = SizeOfG1Compressed * 2

// Bytes returns binary representation of p
// will store X coordinate in regular form and a parity bit
// as we have less than 3 bits available in our coordinate, we can't follow BLS381 style encoding (ZCash/IETF)
// we use the 2 most significant bits instead
// 00 -> uncompressed
// 10 -> compressed, use smallest lexicographically square root of Y^2
// 11 -> compressed, use largest lexicographically square root of Y^2
// 01 -> compressed infinity point
// the "uncompressed infinity point" will just have 00 (uncompressed) followed by zeroes (infinity = 0,0 in affine coordinates)
func (p *G1) Bytes() (res [SizeOfG1Compressed]byte) {

	// check if p is infinity point
	if p.X.IsZero() && p.Y.IsZero() {
		res[0] = mCompressedInfinity
		return
	}

	// tmp is used to convert from montgomery representation to regular
	var tmp fp.Element

	msbMask := mCompressedSmallest
	// compressed, we need to know if Y is lexicographically bigger than -Y
	// if p.Y ">" -p.Y
	if p.Y.LexicographicallyLargest() {
		msbMask = mCompressedLargest
	}

	// we store X  and mask the most significant word with our metadata mask
	tmp = p.X
	tmp.FromMont()
	binary.BigEndian.PutUint64(res[24:32], tmp[0])
	binary.BigEndian.PutUint64(res[16:24], tmp[1])
	binary.BigEndian.PutUint64(res[8:16], tmp[2])
	binary.BigEndian.PutUint64(res[0:8], tmp[3])

	res[0] |= msbMask

	return
}

// RawBytes returns binary representation of p (stores X and Y coordinate)
// see Bytes() for a compressed representation
func (p *G1) RawBytes() (res [SizeOfG1Uncompressed]byte) {

	// check if p is infinity point
	if p.X.IsZero() && p.Y.IsZero() {

		res[0] = mUncompressed

		return
	}

	// tmp is used to convert from montgomery representation to regular
	var tmp fp.Element

	// not compressed
	// we store the Y coordinate
	tmp = p.Y
	tmp.FromMont()
	binary.BigEndian.PutUint64(res[56:64], tmp[0])
	binary.BigEndian.PutUint64(res[48:56], tmp[1])
	binary.BigEndian.PutUint64(res[40:48], tmp[2])
	binary.BigEndian.PutUint64(res[32:40], tmp[3])

	// we store X  and mask the most significant word with our metadata mask
	tmp = p.X
	tmp.FromMont()
	binary.BigEndian.PutUint64(res[24:32], tmp[0])
	binary.BigEndian.PutUint64(res[16:24], tmp[1])
	binary.BigEndian.PutUint64(res[8:16], tmp[2])
	binary.BigEndian.PutUint64(res[0:8], tmp[3])

	res[0] |= mUncompressed

	return
}

// SetBytes sets p from binary representation in buf and returns number of consumed bytes
// bytes in buf must match either RawBytes() or Bytes() output
// if buf is too short io.ErrShortBuffer is returned
// if buf contains compressed representation (output from Bytes()) and we're unable to compute
// the Y coordinate (i.e the square root doesn't exist) this function retunrs an error
// note that this doesn't check if the resulting point is on the curve or in the correct subgroup
func (p *G1) SetBytes(buf []byte) (int, error) {
	if len(buf) < SizeOfG1Compressed {
		return 0, io.ErrShortBuffer
	}

	// most significant byte
	mData := buf[0] & mMask
	buf[0] &= ^mMask // clear meta data

	// check buffer size
	if mData == mUncompressed {
		if len(buf) < SizeOfG1Uncompressed {
			return 0, io.ErrShortBuffer
		}
	}

	// if infinity is encoded in the metadata, we don't need to read the buffer
	if mData == mCompressedInfinity {
		p.X.SetZero()
		p.Y.SetZero()
		return SizeOfG1Compressed, nil
	}

	// tmp is used to convert to montgomery representation
	var tmp fp.Element

	// read X coordinate
	tmp[0] = binary.BigEndian.Uint64(buf[24:32])
	tmp[1] = binary.BigEndian.Uint64(buf[16:24])
	tmp[2] = binary.BigEndian.Uint64(buf[8:16])
	tmp[3] = binary.BigEndian.Uint64(buf[0:8])
	tmp.ToMont()
	p.X.Set(&tmp)

	// uncompressed point
	if mData == mUncompressed {
		// read Y coordinate
		tmp[0] = binary.BigEndian.Uint64(buf[56:64])
		tmp[1] = binary.BigEndian.Uint64(buf[48:56])
		tmp[2] = binary.BigEndian.Uint64(buf[40:48])
		tmp[3] = binary.BigEndian.Uint64(buf[32:40])
		tmp.ToMont()
		p.Y.Set(&tmp)

		return SizeOfG1Uncompressed, nil
	}

	// we have a compressed coordinate, we need to solve the curve equation to compute Y
	var YSquared, Y fp.Element

	YSquared.Square(&p.X).Mul(&YSquared, &p.X)
	YSquared.Add(&YSquared, &bCurveCoeff)
	if Y.Sqrt(&YSquared) == nil {
		return 0, errors.New("invalid compressed coordinate: square root doesn't exist")
	}

	if Y.LexicographicallyLargest() {
		// Y ">" -Y
		if mData == mCompressedSmallest {
			Y.Neg(&Y)
		}
	} else {
		// Y "<=" -Y
		if mData == mCompressedLargest {
			Y.Neg(&Y)
		}
	}

	p.Y = Y

	return SizeOfG1Compressed, nil
}

// unsafeComputeY called by Decoder when processing slices of compressed point in parallel (step 2)
// it computes the Y coordinate from the already set X coordinate and is compute intensive
func (p *G1) unsafeComputeY() error {
	// stored in unsafeSetCompressedBytes

	mData := byte(p.Y[0])

	// we have a compressed coordinate, we need to solve the curve equation to compute Y
	var YSquared, Y fp.Element

	YSquared.Square(&p.X).Mul(&YSquared, &p.X)
	YSquared.Add(&YSquared, &bCurveCoeff)
	if Y.Sqrt(&YSquared) == nil {
		return errors.New("invalid compressed coordinate: square root doesn't exist")
	}

	if Y.LexicographicallyLargest() {
		// Y ">" -Y
		if mData == mCompressedSmallest {
			Y.Neg(&Y)
		}
	} else {
		// Y "<=" -Y
		if mData == mCompressedLargest {
			Y.Neg(&Y)
		}
	}

	p.Y = Y

	return nil
}

// unsafeSetCompressedBytes is called by Decoder when processing slices of compressed point in parallel (step 1)
// assumes buf[:8] mask is set to compressed
// returns true if point is infinity and need no further processing
// it sets X coordinate and uses Y for scratch space to store decompression metadata
func (p *G1) unsafeSetCompressedBytes(buf []byte) (isInfinity bool) {

	// read the most significant byte
	mData := buf[0] & mMask
	buf[0] &= ^mMask

	if mData == mCompressedInfinity {
		p.X.SetZero()
		p.Y.SetZero()
		isInfinity = true
		return
	}

	// read X

	// tmp is used to convert to montgomery representation
	var tmp fp.Element

	// read X coordinate
	tmp[0] = binary.BigEndian.Uint64(buf[24:32])
	tmp[1] = binary.BigEndian.Uint64(buf[16:24])
	tmp[2] = binary.BigEndian.Uint64(buf[8:16])
	tmp[3] = binary.BigEndian.Uint64(buf[0:8])
	tmp.ToMont()
	p.X.Set(&tmp)

	// store mData in p.Y[0]
	p.Y[0] = uint64(mData)

	// recomputing Y will be done asynchronously
	return
}
