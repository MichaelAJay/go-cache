Key schema (recommended):

data:session:<sid>

meta:session:<sid>

index:session:<sid> → <subjectID> (string)

index:subject:<subjectID> → Set<sid>

Writes:

On create/update: set data:_, meta:_, SET index:session:<sid> <subjectID>, SADD index:subject:<subjectID> <sid>.

Deletes:

By session: run deleteBySessionIDScript.

By subject (small sets): deleteAllSessionsForSubjectScript.

By subject (large sets): loop deleteSessionsForSubjectChunkedScript.
