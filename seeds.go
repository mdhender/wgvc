package wgvc

import "math/rand/v2"

const seedLeafInset = 0.25

func generateProvinceSeeds(count int, random *rand.Rand) []Point {
	seeds := make([]Point, 0, count)
	partitionSeeds(rectangle{max: Point{X: 1, Y: 1}}, count, random, &seeds)
	return seeds
}

func partitionSeeds(leaf rectangle, count int, random *rand.Rand, seeds *[]Point) {
	if count == 1 {
		width := leaf.max.X - leaf.min.X
		height := leaf.max.Y - leaf.min.Y
		*seeds = append(*seeds, Point{
			X: leaf.min.X + width*(seedLeafInset+(1-2*seedLeafInset)*random.Float64()),
			Y: leaf.min.Y + height*(seedLeafInset+(1-2*seedLeafInset)*random.Float64()),
		})
		return
	}

	firstCount := count / 2
	secondCount := count - firstCount
	firstFraction := float64(firstCount) / float64(count)
	width := leaf.max.X - leaf.min.X
	height := leaf.max.Y - leaf.min.Y
	first, second := leaf, leaf
	if width >= height {
		cut := leaf.min.X + width*firstFraction
		first.max.X = cut
		second.min.X = cut
	} else {
		cut := leaf.min.Y + height*firstFraction
		first.max.Y = cut
		second.min.Y = cut
	}
	partitionSeeds(first, firstCount, random, seeds)
	partitionSeeds(second, secondCount, random, seeds)
}
