$ ! PROBE50K.COM - PROBE50I's DECK/DOLLARS cases (sections I3, I5), one per
$ ! P1. A deck that never ends goes on to this file's end of file, which
$ ! ends it (the DCL Dictionary), so later cases would be typed as data.
$ GOTO 'P1'
$ UNQUOTED:
$ TYPE SYS$INPUT
$ DECK/DOLLARS=end
$ EOD
end, lowercase
ENDING, uppercase
END
I5 after the deck
$ EXIT
$ QUOTED:
$ TYPE SYS$INPUT
$ DECK/DOLLARS="end"
END, uppercase
end, lowercase
I5 after the deck
$ EXIT
$ BARE:
$ TYPE SYS$INPUT
$ DECK/DOLLARS
$ WRITE SYS$OUTPUT "I5 a $ line in a bare /DOLLARS deck"
$ EOD
I5 after the deck
$ EXIT
$ INDENTED:
$ TYPE SYS$INPUT
$ DECK/DOLLARS="%%"
  %% indented
%% at the start
I5 after the deck
$ EXIT
$ COMMAND:
$ DECK/DOLLARS="%%"
WRITE SYS$OUTPUT "I3 after DECK/DOLLARS: read"
%%
WRITE SYS$OUTPUT "I3 after %%: read"
$ EXIT
