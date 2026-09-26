# City Catalog

`city_catalog.json` is generated from the GeoNames `cities15000` extract. The
snapshot was retrieved on 2026-09-27 from the official GeoNames dump server.

Source files:

- https://download.geonames.org/export/dump/cities15000.zip
- https://download.geonames.org/export/dump/countryInfo.txt
- https://download.geonames.org/export/dump/admin1CodesASCII.txt

The converter keeps cities with a country name and an IANA timezone recognized
by Go's timezone database. It includes GeoNames IDs, display and ASCII names,
country and first-level region labels, timezone IDs, and population values.
Entries are sorted by country, city name, then GeoNames ID.

GeoNames data is provided under CC BY 4.0. Attribution: "City and country data
from GeoNames (https://www.geonames.org/), licensed under CC BY 4.0
(https://creativecommons.org/licenses/by/4.0/)." GeoNames aggregates many
sources and provides the data as-is, without warranty of accuracy, timeliness,
or completeness.

To regenerate, download the three source files above and run:

```sh
go run ./tools/citycatalog \
  -cities cities15000.zip \
  -countries countryInfo.txt \
  -admin1 admin1CodesASCII.txt \
  -output data/city_catalog.json
```