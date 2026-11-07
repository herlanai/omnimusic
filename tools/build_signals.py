"""Usage: python3 tools/build_signals.py 2012-05.parquet 2016-10.parquet 2024-01.parquet > assets/signals.json
Requires DuckDB. Source URLs and attribution are in README.md.
"""
import json
from pathlib import Path
import sys

import duckdb

db = duckdb.connect()
db.read_parquet(sys.argv[1:]).create_view("items")
query = Path(__file__).with_name("music_signals.sql").read_text()
result = db.execute(query)
columns = [field[0] for field in result.description]
json.dump([dict(zip(columns, row)) for row in result.fetchall()], sys.stdout,
          ensure_ascii=False, indent=1)
print()
