#!/usr/bin/env python3
"""
Assertions over ssosync JSON debug logs for the user/group operations live test
(issue #383), as `check <subcommand> LOG ...` in lib.sh. A front end to the
shared tests/lib/ssolog.py, which documents the subcommands.
"""

import os
import sys

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "lib"))
from ssolog import main  # noqa: E402

if __name__ == "__main__":
    main()
