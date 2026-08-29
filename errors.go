package sqlwhere

import "errors"

// Sentinel errors returned by Build. Use errors.Is.
var (
	ErrInvalidIdent       = errors.New("sqlwhere: invalid identifier")
	ErrEmptyIn            = errors.New("sqlwhere: empty IN list")
	ErrEmptyBool          = errors.New("sqlwhere: empty AND/OR list")
	ErrNilPredicate       = errors.New("sqlwhere: nil predicate")
	ErrEmptySQL           = errors.New("sqlwhere: empty SQL")
	ErrUnknownDialect     = errors.New("sqlwhere: unknown dialect")
	ErrOrderConflict      = errors.New("sqlwhere: base SQL already has ORDER BY")
	ErrLimitConflict      = errors.New("sqlwhere: base SQL already has LIMIT")
	ErrOffsetConflict     = errors.New("sqlwhere: base SQL already has OFFSET")
	ErrBindCount          = errors.New("sqlwhere: Bind arg count does not match placeholders in base SQL")
	ErrMixedPlaceholders  = errors.New("sqlwhere: base SQL mixes $n and ? placeholders")
	ErrDialectPlaceholder = errors.New("sqlwhere: base SQL placeholders do not match Build dialect")
	ErrRawArgs            = errors.New("sqlwhere: Raw placeholder count does not match args")
	ErrNegativeLimit      = errors.New("sqlwhere: LIMIT must be >= 0")
	ErrNegativeOffset     = errors.New("sqlwhere: OFFSET must be >= 0")
	ErrCompoundQuery      = errors.New("sqlwhere: cannot attach clauses to UNION/EXCEPT/INTERSECT, INSERT, or UPDATE ... FROM")
	ErrILikeDialect       = errors.New("sqlwhere: ILIKE requires Postgres dialect")
)
