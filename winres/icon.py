# Draws the vauth-desktop icon: a black padlock with a yellow keyhole on a
# yellow rounded tile. Regenerates icon.png (1024 px) and icon.ico.
from PIL import Image, ImageDraw

S, YELLOW, BLACK = 1024, "#FFD814", "#111111"
image = Image.new("RGBA", (S, S), (0, 0, 0, 0))
draw = ImageDraw.Draw(image)
draw.rounded_rectangle((0, 0, S - 1, S - 1), radius=S * 0.18, fill=YELLOW)
# Shackle: half circle and two legs, behind the body.
cx, top, half, width = S / 2, 150, 210, 95
draw.arc((cx - half, top, cx + half, top + 2 * half), 180, 360, fill=BLACK, width=width)
draw.rectangle((cx - half, top + half, cx - half + width, 520), fill=BLACK)
draw.rectangle((cx + half - width, top + half, cx + half, 520), fill=BLACK)
draw.rounded_rectangle((210, 440, 814, 880), radius=70, fill=BLACK)
# Keyhole.
cy, r = 600, 68
draw.ellipse((cx - r, cy - r, cx + r, cy + r), fill=YELLOW)
draw.polygon([(cx - r * .55, cy + r * .3), (cx + r * .55, cy + r * .3), (cx + r * .8, cy + r * 2.6), (cx - r * .8, cy + r * 2.6)], fill=YELLOW)
image.save("icon.png")
image.resize((256, 256), Image.LANCZOS).save("icon.ico", sizes=[(s, s) for s in (16, 20, 24, 32, 40, 48, 64, 128, 256)])
