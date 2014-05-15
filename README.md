This is a little utility I'm trying to use to automagically rotate and crop the
black borders off of pages straight from my scanner. It works decently for
clean pages with lots of white around the edges but breaks down when there's a
lot of ink that goes straight up to the edge. It does not transform the image,
but rather produces a transformation plan that, if followed, will probably
straighten and crop it decently.

There are probably countless improvements and optimizations to be made. The
former will come with time; the latter does not matter much as the time taken
to analyze is much less than the actual time it takes to shell out to
ImageMagick or GraphicsMagick for the actual transformation.

It is presented as a package; a standalone tool can be found in
/autocrop/main.go. For now it just takes a single image argument (and some
optional flags) and returns the ImageMagick command line string needed to
process the image.
