"""Generate PWA icons for Management Server"""
from PIL import Image, ImageDraw, ImageFont
import math
import os

SIZES = [72, 96, 128, 144, 152, 192, 384, 512]
OUT_DIR = os.path.dirname(os.path.abspath(__file__))

def generate_icon(size):
    img = Image.new('RGBA', (size, size), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # Background rounded rect with gradient effect
    # Since Pillow doesn't support gradients natively, use solid color
    r = int(size * 0.15)
    draw.rounded_rectangle([0, 0, size - 1, size - 1], radius=r, fill=(99, 102, 241))

    # Add subtle darker overlay on bottom-right for depth
    for y in range(size):
        for x in range(size):
            px = img.getpixel((x, y))
            if px[3] > 0:  # Only modify non-transparent pixels
                factor = 0.85 + 0.15 * (1 - (x + y) / (2 * size))
                img.putpixel((x, y), (
                    int(px[0] * factor),
                    int(px[1] * factor),
                    int(px[2] * factor),
                    px[3]
                ))

    draw = ImageDraw.Draw(img)

    # Signal arcs
    cx = int(size * 0.40)
    cy = int(size * 0.50)
    white = (255, 255, 255, 230)

    arc_radii = [int(size * 0.10), int(size * 0.18), int(size * 0.26)]
    widths = [max(2, int(size * 0.03)), max(2, int(size * 0.025)), max(2, int(size * 0.02))]
    for rad, w in zip(arc_radii, widths):
        bbox = [cx - rad, cy - rad, cx + rad, cy + rad]
        draw.arc(bbox, start=-135, end=-15, fill=white, width=w)

    # Center dot
    dot_r = max(2, int(size * 0.04))
    draw.ellipse([cx - dot_r, cy - dot_r, cx + dot_r, cy + dot_r], fill=(255, 255, 255, 245))

    # "NMS" text
    font_size = max(10, int(size * 0.16))
    try:
        font = ImageFont.truetype("arial.ttf", font_size)
    except (OSError, IOError):
        try:
            font = ImageFont.truetype("Arial Bold.ttf", font_size)
        except (OSError, IOError):
            font = ImageFont.load_default()

    text = "NMS"
    text_bbox = draw.textbbox((0, 0), text, font=font)
    tw = text_bbox[2] - text_bbox[0]
    th = text_bbox[3] - text_bbox[1]
    tx = int(size * 0.62) - tw // 2
    ty = int(size * 0.72)
    draw.text((tx, ty), text, fill=(255, 255, 255, 240), font=font)

    return img


if __name__ == '__main__':
    for s in SIZES:
        icon = generate_icon(s)
        path = os.path.join(OUT_DIR, f'icon-{s}x{s}.png')
        icon.save(path, 'PNG')
        print(f'Generated: icon-{s}x{s}.png')

    # Also generate apple-touch-icon (180x180)
    apple = generate_icon(180)
    apple.save(os.path.join(OUT_DIR, 'apple-touch-icon.png'), 'PNG')
    print('Generated: apple-touch-icon.png')

    # Favicon 32x32
    fav = generate_icon(32)
    fav.save(os.path.join(OUT_DIR, 'favicon-32x32.png'), 'PNG')
    print('Generated: favicon-32x32.png')

    # Favicon 16x16
    fav16 = generate_icon(16)
    fav16.save(os.path.join(OUT_DIR, 'favicon-16x16.png'), 'PNG')
    print('Generated: favicon-16x16.png')

    print('\nAll icons generated successfully!')
