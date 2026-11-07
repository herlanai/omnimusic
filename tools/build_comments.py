"""Extract the archived replies to the stories in signals.json from the same Parquet shards."""
import json
import sys
from collections import defaultdict
from html.parser import HTMLParser
from pathlib import Path

import duckdb


class CommentText(HTMLParser):
    def __init__(self):
        super().__init__()
        self.parts = []

    def handle_starttag(self, tag, attrs):
        if tag in ("br", "p", "pre"):
            self.parts.append("\n")

    def handle_data(self, text):
        self.parts.append(text)


assets = Path(__file__).resolve().parents[1] / "assets"
stories = {s["id"] for s in json.loads((assets / "signals.json").read_text())}
db = duckdb.connect()
db.read_parquet(sys.argv[1:]).create_view("items")
rows = db.execute("""
    WITH RECURSIVE tree AS (
        SELECT id, id AS story FROM items WHERE id IN (SELECT unnest(?))
        UNION ALL
        SELECT i.id, t.story FROM items i JOIN tree t ON i.parent=t.id WHERE i.type=2
    )
    SELECT t.story, i.id, i."by", i.text FROM tree t JOIN items i ON i.id=t.id
    WHERE i.type=2 AND i.dead=0 AND i.deleted=0 AND length(trim(i.text))>0
    ORDER BY t.story, i.id
""", [sorted(stories)]).fetchall()
comments = defaultdict(list)
for story, number, author, raw in rows:
    parser = CommentText()
    parser.feed(raw)
    parser.close()
    body = "".join(parser.parts).strip()
    if body and body not in ("[deleted]", "[dead]"):
        comments[story].append(dict(id=number, author=author, body=body))
(assets / "comments.json").write_text(json.dumps(comments, ensure_ascii=False))
print(sum(map(len, comments.values())), "comments in", len(comments), "threads")
