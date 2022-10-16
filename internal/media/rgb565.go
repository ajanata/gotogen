package media

import (
	"image/color"
)

type RGB565 uint16

// copied from https://github.com/ev3go/ev3dev/blob/master/LICENSE
// Copyright ©2016 The ev3go Authors. All rights reserved.
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are met:
// * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
// * Redistributions in binary form must reproduce the above copyright
// notice, this list of conditions and the following disclaimer in the
// documentation and/or other materials provided with the distribution.
// * The name of the author may not be used to endorse or promote products
// derived from this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE
// FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
// DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER
// CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY,
// OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

const (
	rwid = 5
	gwid = 6
	bwid = 5

	boff = 0
	goff = boff + bwid
	roff = goff + gwid

	rmask = 1<<rwid - 1
	gmask = 1<<gwid - 1
	bmask = 1<<bwid - 1

	bytewid = 8
)

func (c RGB565) RGBA() color.RGBA {
	r := uint32(c&(rmask<<roff)) >> (roff - (bytewid - rwid)) // Shift to align high bit to bit 7.
	r |= r >> rwid                                            // Adjust by highest 3 bits.
	r |= r << bytewid

	g := uint32(c&(gmask<<goff)) >> (goff - (bytewid - gwid)) // Shift to align high bit to bit 7.
	g |= g >> gwid                                            // Adjust by highest 2 bits.
	g |= g << bytewid

	b := uint32(c&bmask) << (bytewid - bwid) // Shift to align high bit to bit 7.
	b |= b >> bwid                           // Adjust by highest 3 bits.
	b |= b << bytewid

	cc := color.RGBA{
		R: uint8(r),
		G: uint8(g),
		B: uint8(b),
		A: 0xFF,
	}
	return cc
}
