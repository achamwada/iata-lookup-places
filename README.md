# iata-lookup-places

A tiny Go helper library + tool for working with IATA airport codes using
data from [OurAirports](https://ourairports.com/).

- Downloads `airports.csv` from OurAirports.
- Keeps a timestamped copy **and** a stable `airports-latest.csv`.
- Provides a fast, in-memory `LookupIATA` function you can use in other services.

---

## Installation

```bash
go get github.com/yourname/iata-lookup-places
