# Handling imports in Linter

How should I treat graph parts from imports? Which part of code should be responsible for getting them?

How do I avoid reporting warning in 3rd party code (libraries and such).
For now I can just report only in the explicitly passed file.

And that brings us to the next point. We probably want to lint multiple files and not do the whole
work from the beginning every time, so perhaps linter state which keeps a cache of imported libs and such.
Perhaps files themselves could also be cached (somewhat unlike the way it works normally)

Such a stateful Linter could be pretty neat. There is a problem however - it would need to contain VM
and as such we get cache flushing problems. Perhaps "observer pattern" with VM informing registered things of its invalidation.

Actually stateful Linter causes quite a lot of problems. The graph as currently implemented really doesn't like being extended multiple times. It is better to collect all of the files for which we want to do linting and then handle them in one go. This complicates the API a bit, but internally it's much easier to understand what's going on.

We need to load the imports somehow, when we add file, all of the transitive depenencies...
