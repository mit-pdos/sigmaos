import os

is_forking = os.environ.get("SIGMA_FORK_ZYGOTE_KEY") is not None

if is_forking:
    from splib.fork import fork_point


import sys
import io
from PIL import Image
import splib
import uuid


if is_forking:
    args = fork_point()
#     in_path = args[0]
#     out_path = args[1]
# else:
#     in_path = sys.argv[1]
#     out_path = sys.argv[2]


def main(in_path, out_path):
    splib.started()

    # img_bytes = splib.get_file(in_path)
    # img = Image.open(io.BytesIO(img_bytes))

    img = Image.open(os.path.join(os.path.dirname(__file__), "1.jpg"))

    sizes = [(1920, 1080), (1280, 720), (640, 360), (320, 180)]
    basename = os.path.basename(in_path)
    basename = os.path.splitext(basename)[0]
    basename += f"_{uuid.uuid4().hex[:8]}"

    for w, h in sizes:
        thumb = img.resize((w, h), Image.LANCZOS)

        buf = io.BytesIO()
        thumb.save(buf, format="JPEG", quality=75)
        data = buf.getvalue()

        thumb_out_path = os.path.join(out_path, f"{basename}_{w}x{h}.jpg")
        splib.put_file(thumb_out_path, 0o644, 0, data, 0, 0)

    splib.exited(splib.Status.Ok, "ok")


if __name__ == "__main__":
    in_path = sys.argv[1]
    out_path = sys.argv[2]

    main(in_path, out_path)
