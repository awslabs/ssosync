#!/usr/bin/env python3
"""
Local tests for lambda_logs.select_invocation, using the shape of the real
failure: a second invocation's --start window that also caught the end of
the first invocation.

  python3 cicd/tests/attribute_map/lib/test_lambda_logs.py
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from lambda_logs import done_without_request_id, select_invocation  # noqa: E402

FIRST, SECOND = "372d1c21-584a-49ce-9601-d7ab387a5d38", "94b7de54-357e-48e6-8af1-6535dee7c7c7"


def invocation(rid, summary):
    return [
        f"START RequestId: {rid} Version: $LATEST",
        '{"level":"info","msg":"syncing"}',
        f'{{"level":"info","msg":"Change Summary: Users","adoptLocal":{summary}}}',
        f"END RequestId: {rid}",
        f"REPORT RequestId: {rid}\tDuration: 1000 ms",
    ]


class SelectInvocation(unittest.TestCase):
    def test_keeps_only_the_requested_invocation(self):
        window = invocation(FIRST, 4)[2:] + invocation(SECOND, 0)
        lines, complete = select_invocation(window, SECOND)
        self.assertTrue(complete)
        self.assertEqual(invocation(SECOND, 0), lines)
        self.assertFalse(any(FIRST in l for l in lines))
        self.assertFalse(any('"adoptLocal":4' in l for l in lines))

    def test_waits_for_the_report_line(self):
        partial = invocation(FIRST, 4) + invocation(SECOND, 0)[:3]
        lines, complete = select_invocation(partial, SECOND)
        self.assertFalse(complete, "must keep polling until this invocation's REPORT line")
        self.assertEqual(invocation(SECOND, 0)[:3], lines)

    def test_not_started_yet(self):
        lines, complete = select_invocation(invocation(FIRST, 4), SECOND)
        self.assertEqual(([], False), (lines, complete))

    def test_old_marker_would_have_stopped_early(self):
        # The previous behaviour: any Change Summary line ends the wait, even
        # the previous invocation's (the cause of the LambdaDefaultMap failure).
        window = invocation(FIRST, 4) + invocation(SECOND, 0)[:1]
        self.assertTrue(done_without_request_id(window))
        self.assertFalse(select_invocation(window, SECOND)[1])


if __name__ == "__main__":
    unittest.main()
