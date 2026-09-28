# How to regenerate the README maps and doc images

Every image in the README and `docs/` comes from a command in this guide.
Each command is complete: copy it as written. The flags that are easy to
forget, such as `-islands`, are the ones that change the map most.

## Before you start

Build the generator once so the recipes run from a binary rather than `go run`:

```sh
go build -o /tmp/wgvc-generate ./cmd/generate
```

Every map recipe below pins the same seed, `0x0123456789abcdef`, and 10,000
land provinces. Each one also sets `-islands` explicitly. A constellation
places one attractor per star but does not set the island count, so leaving
`-islands` out seeds the default of 15 islands and produces a different map.

The default `-scale` is 3. The dimensions and file sizes quoted below are at
that scale. To reproduce the smaller images that older releases wrote, add
`-scale 1`; the map is otherwise identical.

Check each run against the status line shown with it. The island counts and
cell counts must match exactly; if they do not, a flag is missing or wrong.

## Regenerate the Subaru archipelago (the README header map)

```sh
/tmp/wgvc-generate -seed 0x0123456789abcdef \
  -islands 21 -provinces 10000 -ocean 0.78 \
  -attractors subaru -format png -output subaru
```

Expected status line:

```
wrote subaru (16325×6617): 45455 cells, 10000 land, 21→6 islands, merges=15, 78% ocean, 1 round(s)
```

The PNG is about 47 MB. See [Refresh the README header](#refresh-the-readme-header)
to turn it into `docs/tnyc.png`.

## Regenerate the TNYC goal map

```sh
/tmp/wgvc-generate -seed 0x0123456789abcdef \
  -islands 27 -provinces 10000 -ocean 0.78 \
  -aspect cinematic -attractors 9 -format png -output tnyc
```

Expected status line:

```
wrote tnyc (15965×6764): 45455 cells, 10000 land, 27→26 islands, merges=1, 78% ocean, 1 round(s)
```

## Regenerate Draco

Draco supplies its own aspect ratio and ocean fraction, so neither `-aspect`
nor `-ocean` is given.

```sh
/tmp/wgvc-generate -seed 0x0123456789abcdef \
  -islands 35 -provinces 10000 \
  -attractors draco -format png -output draco
```

Expected status line:

```
wrote draco (17249×9766): 71429 cells, 10000 land, 35→1 islands, merges=34, 86% ocean, 1 round(s)
```

The PNG is about 74 MB, the largest of the set.

## Regenerate Aster

Aster also supplies its own aspect ratio and ocean fraction.

```sh
/tmp/wgvc-generate -seed 0x0123456789abcdef \
  -islands 25 -provinces 10000 \
  -attractors aster -format png -output aster
```

Expected status line:

```
wrote aster (16144×9144): 62500 cells, 10000 land, 25→14 islands, merges=11, 84% ocean, 1 round(s)
```

## Regenerate the Little Dipper

```sh
/tmp/wgvc-generate -seed 0x0123456789abcdef \
  -islands 7 -provinces 10000 -ocean 0.78 \
  -attractors ursa-minor -format png -output ursa-minor
```

Expected status line:

```
wrote ursa-minor (14617×7381): 45455 cells, 10000 land, 7→7 islands, merges=0, 78% ocean, 1 round(s)
```

## Regenerate the single continent

The continent disables the rival ramp so its 25 islands merge.

```sh
/tmp/wgvc-generate -seed 0x0123456789abcdef \
  -islands 25 -provinces 10000 -ocean 0.45 -rival-ramp 0 \
  -aspect cinematic -attractors 5 -format png -output tnyc-continent
```

Expected status line:

```
wrote tnyc-continent (10151×4331): 18182 cells, 10000 land, 25→1 islands, merges=24, 45% ocean, 1 round(s)
```

## Refresh the README header

The header `docs/tnyc.png` is the Subaru map resized to 1600 pixels wide and
reduced to a 256-colour palette so the repository stays small. With
`subaru.png` from the recipe above:

```sh
magick subaru.png -resize 1600x -colors 256 docs/tnyc.png
```

Confirm the result before committing:

```sh
magick identify -verbose docs/tnyc.png | grep -E 'Geometry|Type'
```

The output must report a geometry 1600 pixels wide and type `Palette`. Then update the
"downscaled from" dimensions in the README's header caption to match the
status line of the render you used.

## Regenerate the doc galleries and terrain legend

These SVGs and the terrain legend are golden outputs written by tests. Each
test rewrites its file only when its environment variable is set, and fails
when the checked-in file is stale otherwise.

```sh
WGVC_UPDATE_SINGLE_MESH_GALLERY=1 go test -run TestSingleMeshIslandGallery      # docs/blob-islands.svg
WGVC_UPDATE_TERRAIN_LEGEND=1     go test -run TestTerrainLegend ./cmd/generate  # docs/references/terrain-legend.png
```

Run the full suite afterwards to confirm nothing else depended on the old
output:

```sh
go test ./...
```

## Render one of these maps at a different size

Add `-scale` to any recipe. The scale multiplies the image size, the margin,
and every stroke width, so the map's content is unchanged:

```sh
/tmp/wgvc-generate -seed 0x0123456789abcdef \
  -islands 21 -provinces 10000 -ocean 0.78 \
  -attractors subaru -scale 1 -format png -output subaru-1x
```

At `-scale 1` the Subaru map is 5442×2206 and about 5 MB.
