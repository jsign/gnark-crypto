// Copyright 2020 ConsenSys AG
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

package bn256

import (
	"github.com/consensys/gurvy/bn256/internal/fptower"
)

// GT target group of the pairing
type GT = fptower.E12

type lineEvaluation struct {
	r0 fptower.E2
	r1 fptower.E2
	r2 fptower.E2
}

// FinalExponentiation computes the final expo x**(p**6-1)(p**2+1)(p**4 - p**2 +1)/r
func FinalExponentiation(z *GT, _z ...*GT) GT {

	var result GT
	result.Set(z)

	for _, e := range _z {
		result.Mul(&result, e)
	}

	// https://eprint.iacr.org/2008/490.pdf
	var mt [4]GT // mt[i] is m^(t^i)

	// easy part
	mt[0].Set(z)
	var temp GT
	temp.Conjugate(&mt[0])
	mt[0].Inverse(&mt[0])
	temp.Mul(&temp, &mt[0])
	mt[0].FrobeniusSquare(&temp).
		Mul(&mt[0], &temp)

	// hard part
	mt[1].Expt(&mt[0])
	mt[2].Expt(&mt[1])
	mt[3].Expt(&mt[2])

	var y [7]GT

	y[1].InverseUnitary(&mt[0])
	y[4].Set(&mt[1])
	y[5].InverseUnitary(&mt[2])
	y[6].Set(&mt[3])

	mt[0].Frobenius(&mt[0])
	mt[1].Frobenius(&mt[1])
	mt[2].Frobenius(&mt[2])
	mt[3].Frobenius(&mt[3])

	y[0].Set(&mt[0])
	y[3].InverseUnitary(&mt[1])
	y[4].Mul(&y[4], &mt[2]).InverseUnitary(&y[4])
	y[6].Mul(&y[6], &mt[3]).InverseUnitary(&y[6])

	mt[0].Frobenius(&mt[0])
	mt[2].Frobenius(&mt[2])

	y[0].Mul(&y[0], &mt[0])
	y[2].Set(&mt[2])

	mt[0].Frobenius(&mt[0])

	y[0].Mul(&y[0], &mt[0])

	// compute addition chain
	var t [2]GT

	t[0].CyclotomicSquare(&y[6])
	t[0].Mul(&t[0], &y[4])
	t[0].Mul(&t[0], &y[5])
	t[1].Mul(&y[3], &y[5])
	t[1].Mul(&t[1], &t[0])
	t[0].Mul(&t[0], &y[2])
	t[1].CyclotomicSquare(&t[1])
	t[1].Mul(&t[1], &t[0])
	t[1].CyclotomicSquare(&t[1])
	t[0].Mul(&t[1], &y[1])
	t[1].Mul(&t[1], &y[0])
	t[0].CyclotomicSquare(&t[0])
	result.Mul(&t[0], &t[1])

	return result
}

// MillerLoop Miller loop
func MillerLoop(P G1, Q G2) *GT {

	var result GT
	result.SetOne()

	if P.IsInfinity() || Q.IsInfinity() {
		return &result
	}

	ch := make(chan struct{}, 30)

	var evaluations [86]lineEvaluation
	var Qjac g2Jac
	Qjac.FromAffine(&Q)
	go preCompute(&evaluations, &Qjac, &P, ch)

	j := 0
	for i := len(loopCounter) - 2; i >= 0; i-- {

		result.Square(&result)
		<-ch
		mulAssign(&result, &evaluations[j])
		j++

		if loopCounter[i] != 0 {
			<-ch
			mulAssign(&result, &evaluations[j])
			j++
		}
	}

	// cf https://eprint.iacr.org/2010/354.pdf for instance for optimal Ate Pairing
	var Q1, Q2 g2Jac

	//Q1 = Frob(Q)
	Q1.X.Conjugate(&Q.X).MulByNonResidue1Power2(&Q1.X)
	Q1.Y.Conjugate(&Q.Y).MulByNonResidue1Power3(&Q1.Y)
	Q1.Z.SetOne()

	// Q2 = -Frob2(Q)
	Q2.X.MulByNonResidue2Power2(&Q.X)
	Q2.Y.MulByNonResidue2Power3(&Q.Y).Neg(&Q2.Y)
	Q2.Z.SetOne()

	var lEval lineEvaluation
	lineEval(&Qjac, &Q1, &P, &lEval)
	mulAssign(&result, &lEval)

	Qjac.AddAssign(&Q1)

	lineEval(&Qjac, &Q2, &P, &lEval)
	mulAssign(&result, &lEval)

	return &result
}

// lineEval computes the evaluation of the line through Q, R (on the twist) at P
// Q, R are in jacobian coordinates
func lineEval(Q, R *g2Jac, P *G1, result *lineEvaluation) {

	// converts _Q and _R to projective coords
	var _Q, _R g2Proj
	_Q.FromJacobian(Q)
	_R.FromJacobian(R)

	result.r1.Mul(&_Q.y, &_R.z)
	result.r0.Mul(&_Q.z, &_R.x)
	result.r2.Mul(&_Q.x, &_R.y)

	_Q.z.Mul(&_Q.z, &_R.y)
	_Q.x.Mul(&_Q.x, &_R.z)
	_Q.y.Mul(&_Q.y, &_R.x)

	result.r1.Sub(&result.r1, &_Q.z)
	result.r0.Sub(&result.r0, &_Q.x)
	result.r2.Sub(&result.r2, &_Q.y)

	result.r1.MulByElement(&result.r1, &P.X)
	result.r0.MulByElement(&result.r0, &P.Y)
}

func mulAssign(z *GT, l *lineEvaluation) *GT {

	var a, b, c GT
	a.MulByVW(z, &l.r1)
	b.MulByV(z, &l.r0)
	c.MulByV2W(z, &l.r2)
	z.Add(&a, &b).Add(z, &c)

	return z
}

// precomputes the line evaluations used during the Miller loop.
func preCompute(evaluations *[86]lineEvaluation, Q *g2Jac, P *G1, ch chan struct{}) {

	var Q1, Qbuf, Qneg g2Jac
	Q1.Set(Q)
	Qbuf.Set(Q)
	Qneg.Neg(Q)

	j := 0

	for i := len(loopCounter) - 2; i >= 0; i-- {

		Q1.Set(Q)
		Q.Double(&Q1).Neg(Q)
		lineEval(&Q1, Q, P, &evaluations[j]) // f(P), div(f) = 2(Q1)+(-2Q)-3(O)
		Q.Neg(Q)
		ch <- struct{}{}
		j++

		if loopCounter[i] == 1 {
			lineEval(Q, &Qbuf, P, &evaluations[j]) // f(P), div(f) = (Q)+(Qbuf)+(-Q-Qbuf)-3(O)
			Q.AddAssign(&Qbuf)
			ch <- struct{}{}
			j++
		} else if loopCounter[i] == -1 {
			lineEval(Q, &Qneg, P, &evaluations[j]) // f(P), div(f) = (Q)+(-Qbuf)+(-Q+Qbuf)-3(O)
			Q.AddAssign(&Qneg)
			ch <- struct{}{}
			j++
		}
	}

	close(ch)
}
