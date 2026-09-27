# About exits, bearings, and compass points

Every province in a `wgvc` world lists its boundary edges as numbered
*exits*, each with a bearing in degrees and an eight-point compass label.
This page explains why the numbering works the way it does, why the bearing
is the authoritative direction and the compass label is only a courtesy, and
what the design deliberately leaves out.

## Where the numbering comes from

The exits exist for T'Nyc, whose movement orders name a direction by number
(`MOVE 5`) and whose turn reports list "direction 5, South, is Hills". The
original game ran on a map where eight compass points were enough: north was
always 1, south was always 5, and the report simply omitted directions that
had no road.

A `wgvc` province is a Voronoi cell, and Voronoi cells do not have eight
sides. In a typical world they have anywhere from four to ten, and land
provinces alone range from four to nine. A fixed eight-slot scheme has no
honest answer for a nine-sided province, and even a six-sided one often has
two neighbors inside the same 45-degree wedge. So the generator numbers exits
densely instead: a province with seven edges has exits 1 through 7, no gaps.
Zero is never used, which leaves it free for T'Nyc's `HOLD`, a move in
direction zero.

The numbering is a contract, not a suggestion. For the same seed and
configuration, exit 3 of province 212 is the same edge in every build of a
given generator version, because it is derived entirely from geometry and
consumes no randomness.

## Why clockwise from north

Polygon rings in `wgvc` run counterclockwise in Cartesian coordinates with
+Y pointing north, the same convention the image renderers flip into screen
space. Walking a ring in its stored order therefore visits the exits in the
order north, west, south, east. That is backwards from how people read a
compass, and backwards from the T'Nyc rules, where 2 lies between 1 (north)
and 3 (east).

Exits therefore run clockwise, which means reverse ring order. Exit 1 is the
edge whose outward bearing is the smallest, in other words the first exit
clockwise from due north, and the numbers increase as you turn to the right.
Because every cell is convex and its center lies inside it, the outward
bearings increase strictly with the exit number. A player who knows exit 1
faces roughly north-northeast can guess that the southern exit is somewhere in
the middle of the list.

Starting at the province's lowest corner ID would also be deterministic, and
it is how `CornerIDs` and `EdgeIDs` are anchored. It was rejected for exits
because the lowest corner ID is an accident of coordinate sorting that tells
a player nothing about where that exit points.

## Why the bearing is the standard

Each exit carries a bearing in degrees clockwise from north, in `[0, 360)`.
It is the direction of the edge's outward normal. This is a better choice
than the direction from the center to the edge's midpoint, and it is the same
as the direction to the neighbor's center, for a reason particular to Voronoi
cells: the shared edge lies on the perpendicular bisector between the two
generating points, so the perpendicular to the edge points exactly at the
neighbor's center. One definition works for interior and boundary edges alike,
and boundary edges, which have no neighbor, still get a meaningful bearing.

The compass label is derived from the bearing by rounding to the nearest of
eight 45-degree sectors. It is there so a report can print "5 (NE)" and a
player can read it at a glance. It is *not* unique. In the sample world used
while designing this, 193 of 300 provinces had at least two exits sharing a
label, because two neighbors can easily sit within a few degrees of each
other on the same side. A game that resolved `MOVE NE` by label would have to
pick one arbitrarily. Code should therefore treat the bearing, or better the
exit number, as the identity of a direction, and the label as decoration.

## Boundary exits and the shape of the world

An edge on the outside of the world map has one incident province and no
neighbor. Such edges still appear as exits, with the neighbor set to
`NoProvinceID`, rather than being skipped. The numbering stays purely
geometric that way: if a later version of the game decides some coastline is
impassable, or some ocean edge leads somewhere, the exit numbers do not shift
underneath existing orders. In practice boundary exits only occur on ocean
provinces, because the growth stage keeps land away from the map edge.

It would be pleasant for a T'Nyc map to wrap east to west, so that sailing
off the right edge arrives at the left. The generator does not do this, and
does not intend to. The Voronoi tessellation is computed inside a flat
rectangle, so the cells along the left edge and the cells along the right
edge were never neighbors; the tessellation would have to be computed on a
cylinder to know which of them touch, and there is no sound way to pair them
up after the fact. Boundary exits should be read as edges of the known world,
not as portals.

## What exits are not

Exits describe which edges a province has and which way they face. They say
nothing about whether a unit can cross, how long it takes, or whether the far
side is hostile. Those are game rules, and they belong in T'Nyc. Edge
incidence in `wgvc` is geometric adjacency, and an exit is just that
adjacency with a number and a direction attached.
