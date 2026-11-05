package lexer

import (
	"toth/token"
)

type Lexer struct {
	input        string
	position     int
	readPosition int
	ch           byte
	curLine      int
}

func (l *Lexer) readChar() {
	//const readstepSize int = 1
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition += 1
}

func New(input string) *Lexer {
	l := &Lexer{input: input}
	l.curLine = 1
	l.readChar()
	return l
}

func (l *Lexer) NextToken() token.Token {
	var tok token.Token
	l.skipWhitespace()

	if l.ch == '\\' {
		l.skipComment()
	}

	switch l.ch {

	default:
		if isVarter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = token.LookupIdent(tok.Literal)
			//varTok = tok
			//varTokPos = l.position

			//fmt.Println(tok.Type)
			tok.Line = l.curLine
			return tok
		} else if isDigit(l.ch) {
			tok.Literal = l.readNumber()
			if containsDot(tok.Literal) {
				tok.Type = token.FLOAT
			} else {
				tok.Type = token.INT
			}
			tok.Line = l.curLine
			return tok
		} else {
			tok = newToken(token.ILLEGAL, l.ch)
		}

	case '=':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.EQ, Literal: literal}
		} else if l.peekChar() == ':' {
			l.readChar()
			tok = newToken(token.GLOBALASSIGN, l.ch)
		} else {
			tok = newToken(token.ASSIGN, l.ch)
		}
	case '|':
		if l.peekChar() == '|' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.OR, Literal: literal}
		}
	case '&':
		if l.peekChar() == '&' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.AND, Literal: literal}
		}
	case ':':
		tok = newToken(token.COLON, l.ch)
	case ';':
		tok = newToken(token.SEMICOLON, l.ch)
	case ',':
		tok = newToken(token.COMMA, l.ch)
	case '+':
		if l.peekChar() == '=' {
			l.readChar()
			tok = newToken(token.PLUSASSIGN, l.ch)
		} else {
			tok = newToken(token.PLUS, l.ch)
		}
	case '-':
		if l.peekChar() == '=' {
			l.readChar()
			tok = newToken(token.MINUSASSIGN, l.ch)
		} else {
			tok = newToken(token.MINUS, l.ch)
		}
	case '*':
		tok = newToken(token.ASTERISK, l.ch)
	case '%':
		tok = newToken(token.PERC, l.ch)
	case '^':
		tok = newToken(token.CARET, l.ch)
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.NOT_EQ, Literal: literal}
		} else {
			tok = newToken(token.BANG, l.ch)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.LEQ, Literal: literal}
		} else {
			tok = newToken(token.LT, l.ch)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			literal := string(ch) + string(l.ch)
			tok = token.Token{Type: token.GEQ, Literal: literal}
		} else {
			tok = newToken(token.RT, l.ch)
		}
	case '/':
		tok = newToken(token.SLASH, l.ch)
	case '(':
		tok = newToken(token.LPAREN, l.ch)
	case ')':
		tok = newToken(token.RPAREN, l.ch)
	case '{':
		tok = newToken(token.LBRACE, l.ch)
	case '}':
		tok = newToken(token.RBRACE, l.ch)
	case '[':
		tok = newToken(token.LBRACKET, l.ch)
	case ']':
		tok = newToken(token.RBRACKET, l.ch)
	case '"':
		tok.Type = token.STRING
		tok.Literal = l.readString()

	case 0:
		tok.Literal = ""
		tok.Type = token.EOF
	}
	//print(tok.Literal)
	tok.Line = l.curLine
	//fmt.Println(tok.Type)
	l.readChar()
	return tok
}

func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	return l.input[position:l.position]
}

func (l *Lexer) skipComment() {
	for {
		l.readChar()
		if l.ch == '\\' || l.ch == 0 {
			l.readChar()
			l.skipWhitespace()
			break
		}
	}

}

func newToken(tokenType token.TokenType, ch byte) token.Token {
	return token.Token{Type: tokenType, Literal: string(ch)}
}

func (l *Lexer) readIdentifier() string {
	position := l.position
	for isVarterNum(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func isVarter(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_'
}

func isVarterNum(ch byte) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || '0' <= ch && ch <= '9'
}

func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}

func isNumber(ch byte) bool {
	return '0' <= ch && ch <= '9' || ch == '.'
}
func containsDot(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return true
		}
	}
	return false
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\n' || l.ch == '\f' || l.ch == '\t' || l.ch == '\r' {
		if l.ch == '\n' {
			l.curLine++
		}
		l.readChar()
	}
}

func (l *Lexer) readNumber() string {
	position := l.position
	for isNumber(l.ch) {
		l.readChar()
	}
	return l.input[position:l.position]
}

func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	} else {
		return l.input[l.readPosition]
	}
}
