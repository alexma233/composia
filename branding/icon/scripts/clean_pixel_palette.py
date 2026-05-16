from PIL import Image
from collections import Counter
import argparse
import math


def rgb_distance(c1, c2):
    r1, g1, b1 = c1
    r2, g2, b2 = c2

    # Weighted RGB distance, closer to perceived brightness than plain Euclidean.
    return (
        0.30 * (r1 - r2) ** 2 +
        0.59 * (g1 - g2) ** 2 +
        0.11 * (b1 - b2) ** 2
    )


def nearest_color(color, palette):
    return min(palette, key=lambda p: rgb_distance(color, p))


def main():
    parser = argparse.ArgumentParser(
        description="Clean tiny dirty colors from pixel art by mapping them to dominant colors."
    )
    parser.add_argument("input", help="Input PNG")
    parser.add_argument("output", help="Output PNG")
    parser.add_argument(
        "--keep",
        type=int,
        default=16,
        help="Keep top N most-used colors. Default: 16"
    )
    parser.add_argument(
        "--min-count",
        type=int,
        default=None,
        help="Also keep colors used at least this many pixels."
    )
    parser.add_argument(
        "--min-percent",
        type=float,
        default=0.0,
        help="Also keep colors used at least this percent of visible pixels. Example: 0.3"
    )
    parser.add_argument(
        "--drop-alpha",
        type=int,
        default=16,
        help="Pixels with alpha <= this become fully transparent. Default: 16"
    )
    parser.add_argument(
        "--opaque-alpha",
        type=int,
        default=240,
        help="Pixels with alpha >= this become fully opaque. Default: 240"
    )

    args = parser.parse_args()

    img = Image.open(args.input).convert("RGBA")
    w, h = img.size
    pixels = list(img.get_flattened_data())

    cleaned = []

    for r, g, b, a in pixels:
        if a <= args.drop_alpha:
            cleaned.append((0, 0, 0, 0))
        else:
            # Pixel-art mode: every visible pixel becomes fully opaque.
            cleaned.append((r, g, b, 255))

    visible_rgbs = [(r, g, b) for r, g, b, a in cleaned if a > 0]
    counts = Counter(visible_rgbs)

    if not counts:
        raise SystemExit("No visible pixels found.")

    visible_total = len(visible_rgbs)

    top_colors = [color for color, _ in counts.most_common(args.keep)]

    palette = []
    for color in top_colors:
        if color not in palette:
            palette.append(color)

    if args.min_count is not None:
        for color, count in counts.items():
            if count >= args.min_count and color not in palette:
                palette.append(color)

    if args.min_percent > 0:
        min_count_from_percent = math.ceil(visible_total * args.min_percent / 100)
        for color, count in counts.items():
            if count >= min_count_from_percent and color not in palette:
                palette.append(color)

    mapped_cache = {}

    result = []
    for r, g, b, a in cleaned:
        if a == 0:
            result.append((0, 0, 0, 0))
            continue

        rgb = (r, g, b)
        if rgb in palette:
            result.append((r, g, b, 255))
            continue

        if rgb not in mapped_cache:
            mapped_cache[rgb] = nearest_color(rgb, palette)

        nr, ng, nb = mapped_cache[rgb]
        result.append((nr, ng, nb, 255))

    out = Image.new("RGBA", (w, h))
    out.putdata(result)
    out.save(args.output)

    after_counts = Counter((r, g, b) for r, g, b, a in result if a > 0)

    print(f"Size: {w}x{h}")
    print(f"Visible pixels: {visible_total}")
    print(f"Colors before: {len(counts)}")
    print(f"Colors after: {len(after_counts)}")
    print()
    print("Kept palette:")
    for color, count in after_counts.most_common():
        r, g, b = color
        print(f"  #{r:02X}{g:02X}{b:02X}  {count} px")


if __name__ == "__main__":
    main()
