CREATE TABLE discovery_states (
 provider TEXT NOT NULL CHECK(provider='octopus'),
 stream TEXT NOT NULL CHECK(stream IN ('import','export')),
 state TEXT NOT NULL CHECK(state IN ('discovering','selected','unavailable','selection_required','conflict','failed')),
 occurred INTEGER NOT NULL,
 PRIMARY KEY(provider,stream)
);
