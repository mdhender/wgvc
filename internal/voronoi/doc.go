// Package voronoi computes Voronoi diagrams with Fortune's sweep-line
// algorithm.
//
// It is a copy of github.com/pzsz/voronoi at commit 4314be88c79f (itself a port
// of Raymond Hill's Javascript-Voronoi), kept under the original MIT license in
// LICENSE.md. The only change is to the floating-point arithmetic: every
// product that feeds an addition or subtraction is wrapped in an explicit
// float64 conversion. The Go specification lets a compiler fuse x*y + z into a
// single multiply-add with one rounding, and the arm64 backend does while amd64
// does not, so the upstream code produced vertices that differed in their
// low-order bits between the two architectures. An explicit conversion forces
// the product to round on its own, which makes the diagram bit-identical
// everywhere. The unused utils subpackage is not included.
package voronoi
