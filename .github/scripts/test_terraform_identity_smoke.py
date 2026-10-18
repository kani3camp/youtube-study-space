"""Offline regression for the remaining private metadata transport."""
import io
import json
import unittest
from unittest.mock import patch
import terraform_identity_smoke as smoke

class IdentityTransportTest(unittest.TestCase):
    def test_google_transport_preserves_implicit_get_and_body_post_and_allows_empty_post(self):
        body = {'permissions': ['dummy.permission']}
        cases = [(None, {}, 'GET', None), (body, {}, 'POST', json.dumps(body).encode()),
                 (None, {'method': 'POST'}, 'POST', None)]
        for payload, options, method, data in cases:
            with self.subTest(method=method, has_body=payload is not None):
                response = io.BytesIO(b'{}')
                response.status = 200
                with patch.object(smoke.urllib.request, 'urlopen', return_value=response) as http:
                    self.assertEqual(smoke.google('v1/dummy', 'DUMMY_TOKEN', payload,
                                                 host='iam.googleapis.com', **options), (200, {}))
                request = http.call_args.args[0]
                self.assertEqual(request.get_method(), method)
                self.assertEqual(request.data, data)
                self.assertEqual(request.full_url, 'https://iam.googleapis.com/v1/dummy')
                self.assertEqual(http.call_args.kwargs, {'timeout': 30})


if __name__ == '__main__':
    unittest.main()
