# starcat data

`hipparcos.bin` is a reduced extract of the HYG stellar database (which
merges Hipparcos, Yale Bright Star, and Gliese), limited to `mag <= 9.0`
and excluding the Sun (id 0). It's generated **offline, once** — not
fetched or rebuilt at build/CI time — and committed as binary data, the
same way `advice/presets.json` is committed rather than derived at build
time.

## Format

Little-endian binary:

```
uint32              star count N
N x {
  float32  raDeg     right ascension, degrees (0-360)
  float32  decDeg    declination, degrees (-90..90)
  int16    magTenths magnitude * 10 (e.g. 65 = mag 6.5)
  uint32   hip        Hipparcos catalog number, 0 if none
}
```

## Regenerating

Source: HYG database v3 CSV (`hygdata_v3.csv`), e.g.
https://github.com/astronexus/HYG-Database (or a mirror — the upstream repo
moves occasionally; search for `hygdata_v3.csv` if the URL below is stale).

```bash
curl -sL -o hygdata_v3.csv \
  https://raw.githubusercontent.com/astronexus/HYG-Database/master/hygdata_v3.csv

python3 - << 'EOF'
import csv, struct

rows = []
with open("hygdata_v3.csv", newline="", encoding="utf-8") as f:
    for row in csv.DictReader(f):
        if int(row["id"]) == 0:
            continue  # exclude Sol
        ra_h, dec, mag = float(row["ra"]), float(row["dec"]), float(row["mag"])
        if mag > 9.0:
            continue
        hip = int(row["hip"]) if row["hip"] else 0
        rows.append((ra_h * 15.0, dec, mag, hip))

with open("hipparcos.bin", "wb") as f:
    f.write(struct.pack("<I", len(rows)))
    for ra, dec, mag, hip in rows:
        f.write(struct.pack("<ffhI", ra, dec, int(round(mag * 10)), hip))
EOF
```

At time of generation this produced **83,467 stars, ~1.14 MB**.
