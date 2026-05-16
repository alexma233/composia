from PIL import Image
from collections import defaultdict, Counter
from pathlib import Path
import sys


def rect_path(x, y, w):
    # Same rectangle, choose shorter closing command.
    # Mx yhWv1h-Wz
    a = f"M{x} {y}h{w}v1h-{w}z"
    # Mx yhWv1Hxz
    b = f"M{x} {y}h{w}v1H{x}z"
    return a if len(a) <= len(b) else b


def png_to_svg(input_path, output_path):
    img = Image.open(input_path).convert("RGBA")
    width, height = img.size
    px = img.load()

    paths = defaultdict(list)

    for y in range(height):
        x = 0
        while x < width:
            r, g, b, a = px[x, y]

            if a == 0:
                x += 1
                continue

            start = x
            color = (r, g, b, a)

            while x < width and px[x, y] == color:
                x += 1

            run = x - start

            if a == 255:
                attrs = f'fill="#{r:02x}{g:02x}{b:02x}"'
            else:
                opacity = f"{a / 255:.4f}".rstrip("0").rstrip(".")
                attrs = f'fill="#{r:02x}{g:02x}{b:02x}" opacity="{opacity}"'

            paths[attrs].append(rect_path(start, y, run))

    # More stable output: larger colors first, then attributes.
    def path_sort(item):
        attrs, d = item
        return (-len(d), attrs)

    body = "".join(
        f'<path {attrs} d="{"".join(d_parts)}"/>'
        for attrs, d_parts in sorted(paths.items(), key=path_sort)
    )

    svg = (
        f'<svg xmlns="http://www.w3.org/2000/svg" '
        f'viewBox="0 0 {width} {height}" '
        f'shape-rendering="crispEdges">{body}</svg>'
    )

    Path(output_path).write_text(svg, encoding="utf-8")

    visible = [
        (r, g, b, a)
        for r, g, b, a in img.get_flattened_data()
        if a > 0
    ]

    print(f"{input_path} -> {output_path}")
    print(f"size: {width}x{height}")
    print(f"visible pixels: {len(visible)}")
    print(f"colors: {len(Counter(visible))}")
    print(f"paths: {len(paths)}")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print("Usage: python png_to_min_svg.py input.png output.svg")
        sys.exit(1)

    png_to_svg(sys.argv[1], sys.argv[2])
