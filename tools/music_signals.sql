-- Run with a DuckDB "items" view over the chosen OpenIndex monthly Parquet files.
-- Keep one row per discussion and instrumentation, even when several replies match.
WITH RECURSIVE threads AS (
    SELECT id, id AS story, lower(title) AS words
    FROM items WHERE type = 1 AND deleted = 0 AND dead = 0
    AND regexp_matches(lower(title), '\b(music|musical|concert|orchestra|violin|cello|piano|pianist|quartet|beethoven|mozart|symphony|bach canons|synthesizer|composer|musicians|instrument|instruments)\b')
AND NOT regexp_matches(lower(title), 'container orchestrat|anscombe|kubernetes|ibm.s symphony|composer to install|chorded keyboard|piano test for program|orchestra: an oss|cocktail bars|ccp members')
    UNION ALL
    SELECT c.id, t.story, lower(c.text)
    FROM items c JOIN threads t ON c.parent = t.id
    WHERE c.type = 2 AND c.deleted = 0 AND c.dead = 0
), topics(instrument, pattern) AS (VALUES
    ('独立流行', '\b(indie pop|indie rock|pop music|pop song|pop songs|singer.songwriter)\b'),
    ('电子流行', '\b(electropop|synthpop|synth.pop|electronic pop|electronic music)\b'),
    ('钢琴', '\b(piano|pianist|pianos|pianists|harpsichord)\b'),
    ('室内乐', '\b(string quartet|chamber music|violin|cellist|cello|violinist)\b'),
    ('管弦乐', '\b(symphonic music|symphony orchestra|orchestral music|philharmonic|orchestral performance|orchestra|orchestras|symphonies)\b')
), matches AS (
    SELECT story, instrument, count(*) AS mentions
    FROM threads, topics WHERE regexp_matches(words, pattern)
    GROUP BY story, instrument
)
SELECT i.id, i.title, i.score, greatest(i.descendants, 0) AS comments,
       instrument AS "group", mentions
FROM matches JOIN items i ON i.id = story
ORDER BY i.id, instrument
