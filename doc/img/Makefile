AASVG_FILES := $(wildcard *.aasvg)
SVG_FILES := $(AASVG_FILES:.aasvg=.svg)
PNG_FILES := $(AASVG_FILES:.aasvg=.png)

.PHONY: all clean

all: $(SVG_FILES)

%.svg: %.aasvg
	aasvg backdrop spaces=1 < $< > $@

%.png: %.svg
	convert -background white $< > $@

clean:
	rm -f $(SVG_FILES) $(PNG_FILES)
