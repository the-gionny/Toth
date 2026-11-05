package token

var keywords = map[string]TokenType{
	"fn":     FUNCTION,
	"var":    VAR,
	"return": RETURN,
	"if":     IF,
	"while":  WHILE,
	"else":   ELSE,
	"true":   TRUE,
	"false":  FALSE,
	"using":  USING,
	"for":    FOR,
}

func LookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}

const (
	ILLEGAL = "ILLEGAL"
	EOF     = "EOF"

	IDENT = "IDENT"
	INT   = "INT"
	FLOAT = "FLOAT"

	//operators
	MINUSASSIGN  = "-="
	PLUSASSIGN   = "+="
	GLOBALASSIGN = "=:"
	ASSIGN       = "="
	PLUS         = "+"
	MINUS        = "-"
	BANG         = "!"
	ASTERISK     = "*"
	SLASH        = "/"
	PERC         = "%"
	CARET        = "^"

	LT     = "<"
	RT     = ">"
	EQ     = "=="
	LEQ    = "<="
	GEQ    = ">="
	NOT_EQ = "!="
	OR     = "||"
	AND    = "&&"

	//delimeters
	//COMMENT = "@"

	COLON     = ":"
	COMMA     = ","
	SEMICOLON = ";"
	LPAREN    = "("
	RPAREN    = ")"
	LBRACE    = "{"
	RBRACE    = "}"
	LBRACKET  = "["
	RBRACKET  = "]"

	//keywords
	FUNCTION = "FUNCTION"
	VAR      = "VAR"
	IF       = "IF"
	WHILE    = "WHILE"
	ELSE     = "ELSE"
	RETURN   = "RETURN"
	TRUE     = "TRUE"
	FALSE    = "FALSE"
	STRING   = "STRING"
	USING    = "USING"
	FOR      = "FOR"
)

type TokenType string
type Token struct {
	Type    TokenType
	Literal string
	Line    int
}
