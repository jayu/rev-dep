## Ideas

entry point input in the top

Entry points list of the left
  - click entry points starts resolution and shows tree
Resolved files view on center-right
  - three types of views
    - files presented in directory tree based on fs order
    - files presented in directory tree based on resolution tree
      - we might have cycles or repeated sub-trees
        - cycles we mark and not expand
        - repeated subtrees we color code and expand only first occurrence
    - d3 dependency graph
      - files are just nodes, hover on file shows file path in footer
      - clicking node shows options in footer to
        - show resolution path
        - set entry point to file path
  - Icons near file name to
    - show modal with resolution path
    - set entry point to file path
- file filters in both columns

- Figure out what would be useful for node-modules to show in the app0

- Visualize dependencies between all files in given directory (excluding files from other directories - they are not resolved) - to understand what would be the better colocation