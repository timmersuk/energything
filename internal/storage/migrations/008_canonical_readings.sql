-- Canonical samples use the source's reported period, not an attempt identity.
DROP TABLE sample_versions;
DROP TABLE interval_versions;
DROP TABLE aggregate_versions;
CREATE TABLE canonical_samples (
 identity_id INTEGER NOT NULL REFERENCES identities(id),
 provider_date TEXT NOT NULL,
 provider_time TEXT NOT NULL CHECK(length(provider_time)=19 AND substr(provider_time,1,10)=provider_date),
 attempt_id INTEGER NOT NULL REFERENCES attempts(id),
 instant INTEGER, original_time TEXT NOT NULL, quality TEXT NOT NULL,
 values_json TEXT NOT NULL, retrieved INTEGER NOT NULL,
 retrieval_id INTEGER NOT NULL REFERENCES retrievals(id),
 PRIMARY KEY(identity_id,provider_date,provider_time)
);
-- Latest accepted retrieval wins. Unkeyable legacy rows remain in evidence,
-- rather than receiving an invented reading identity.
INSERT INTO canonical_samples
SELECT identity_id,provider_date,clock,attempt_id,instant,original_time,quality,values_json,retrieved,retrieval_id
FROM (
 SELECT *,row_number() OVER(PARTITION BY identity_id,provider_date,clock ORDER BY retrieved DESC,retrieval_id DESC,id DESC) position
 FROM (
  SELECT *,CASE WHEN json_valid(json_extract(original_time,'$.time'))
    THEN json_extract(json_extract(original_time,'$.time'),'$') END clock FROM samples
 ) WHERE length(clock)=19 AND substr(clock,1,10)=provider_date
) WHERE position=1;
DROP TABLE samples;
ALTER TABLE canonical_samples RENAME TO samples;
CREATE INDEX samples_source_time ON samples(identity_id,instant);
CREATE INDEX samples_time ON samples(instant);
CREATE INDEX samples_quality_date ON samples(quality,provider_date,retrieved);
CREATE INDEX samples_retrieved ON samples(retrieved);
