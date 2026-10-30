// Package lex is the FORTRAN compiler's second pass, its lexer: it decides
// what kind of statement each statement is and splits it into tokens, the
// words the parser reads (docs/PHASE-75 - FORTRAN first light.md, Part
// B, and Decision 5 of docs/PHASE-74 - FORTRAN plan.md).
//
// # Why FORTRAN's lexer is unusual
//
// In most languages the lexer runs first and knows nothing of grammar: it
// reads characters, and whenever a word ends it hands the parser a token.
// It can do that because blanks separate words and keywords are reserved:
// "while" is always the keyword, never a variable.
//
// FORTRAN has neither rule. Blanks mean nothing outside character
// constants, so GO TO, GOTO, and G O T O are the same, and keywords are
// ordinary names everywhere else: a program may have variables named DO,
// IF, and END. The classic example (the manual's section 5.3.1):
//
//	DO 10 I = 1,5      a DO loop, ending at label 10
//	DO 10 I = 1.5      DO10I = 1.5, an assignment to a variable DO10I
//
// The two differ in one character, the comma, and it comes after the
// part a conventional lexer would have had to decide on. So this lexer
// works the other way round, in three steps:
//
//  1. Squeeze (squeeze.go): take out every blank outside character and
//     Hollerith constants, and make letters capitals. "DO 10 I = 1, 5"
//     becomes "DO10I=1,5". Each character remembers where it came from,
//     so a message can still point at the right column.
//
//  2. Classify (lex.go's classify): look at the whole statement and
//     decide its kind, using what is at parenthesis level 0 (outside every
//     parenthesis, and every constant):
//
//     - If it starts IF( or ELSEIF(, find the parenthesis that closes the
//     condition. If = follows it, the statement is an assignment to an
//     element of an array named IF: IF(X) = 3. Otherwise what follows
//     says which IF it is: THEN alone, a block IF; digits, an arithmetic
//     IF (IF (X) 10,20,30); anything else, a logical IF, whose own
//     statement is classified in turn: IF (X) GO TO 10. The manual's own
//     warning is one more example: IF (E) THEN I = J is the logical IF
//     IF (E) THENI = J.
//
//     - If there is an = at level 0, the statement is an assignment,
//     unless it starts with DO and a comma at level 0 follows the =:
//     then it is a DO loop. That is the test that tells the two DO
//     statements above apart. CALLX = 1 is an assignment too, to CALLX.
//
//     - Otherwise it is a keyword statement, and the keyword is the
//     longest one the text starts with: ENDIF must be found before END,
//     ELSEIF before ELSE (kinds.go's table has every VAX FORTRAN
//     statement, so one the compiler can't compile yet is still known
//     by name).
//
//     One case needs more than the text: INTEGER FUNCTION F(10) and
//     INTEGER FUNCTIONF(10) squeeze to the same thing, and the second is
//     a declaration of an array named FUNCTIONF. A FUNCTION statement can
//     only be a program unit's first statement, so only there is such
//     text taken for one. The lexer knows where units start because END
//     ends them.
//
//     Appendix A.4's PARAMETER statement has the same trouble: PARAMETER
//     PI=3.14 squeezes to PARAMETERPI=3.14, an assignment to a variable
//     PARAMETERPI by the = test. So before a unit's first executable
//     statement, where an assignment can't yet be, text that is
//     PARAMETER, a name, and the = is the PARAMETER statement
//     (oldParameter; our rule, unconfirmed on VMS). OPTIONS /CHECK=ALL
//     has an = too, and is never an assignment.
//
//  3. Tokenize (tokenize.go): with the kind known, split the text after
//     the keyword into tokens: names, numbers, constants, and operators.
//     A name runs as far as letters and digits go, which is safe only
//     because the keyword has already been taken off: in DO10I=1,5 the
//     tokenizer sees 10I=1,5.
//
//     Numbers have their own puzzle. In X.EQ.1.AND.Y the period after 1
//     starts the operator .AND., but in 1.5 it is a decimal point, so a
//     period after digits is a decimal point only if it doesn't start a
//     dotted operator. And the kind helps here too: DO 10 E1 = 1, 5
//     squeezes to DO10E1=1,5, where 10E1 looks like the real constant
//     100.0; knowing a DO statement starts with a label, the tokenizer
//     takes only the digits. REAL*8 D1 (REAL*8D1) is the same puzzle,
//     solved the same way.
//
// # An example
//
// The statement
//
//	10    IF (I .GT. 0) PRINT *, 'I = ', I
//
// squeezes to IF(I.GT.0)PRINT*,'I = ',I (the blanks in the character
// constant stay), is classified as a logical IF because what follows the
// condition is neither = nor THEN nor a label, and comes out as
//
//	10 logical IF: ( I .GT. 0 ) / PRINT: * , 'I = ' , I
//
// (lex.Dump's form): the condition's tokens, then the PRINT statement it
// controls, lexed as a statement of its own.
//
// # What this pass hands on
//
// Lex returns a Statement for each statement: its Kind, its Label, and
// its Tokens, each Token with its kind, its text, and the offset in the
// statement's text it came from (Statement.Position turns that into a
// file, line, and column). The parser chooses a function by the Kind and
// reads the tokens. The lexer reports only what it can see: a character
// constant never closed (NOQUOTE), a Hollerith constant cut short
// (HOLLEND), a character FORTRAN doesn't use (BADCHAR), and a statement
// it can't classify (UNRECSTMT). A statement the compiler can't compile
// yet is the parser's to report, by its kind's name.
package lex
