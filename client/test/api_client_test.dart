import 'package:flutter_test/flutter_test.dart';
import 'package:snowradar_client/services/api_client.dart';

void main() {
  group('ApiClient', () {
    test('connect throws PaywallRequired on 402', () async {
      final client = ApiClient(
        baseUrl: 'http://127.0.0.1:1', // nothing listens here
        httpClient: _FakeClient((req) async => http.Response('{"error":"x"}', 402)),
      );
      expect(
        () => client.connect(accessToken: 't', nodeId: 'sgp', publicKey: 'k'),
        throwsA(isA<PaywallRequired>()),
      );
    });

    test('refresh endpoint parse', () async {
      final client = ApiClient(
        baseUrl: 'http://127.0.0.1:1',
        httpClient: _FakeClient(
          (req) async => http.Response(
            '{"access_token":"a","refresh_token":"r"}',
            200,
            headers: {'content-type': 'application/json'},
          ),
        ),
      );
      final (a, r) = await client.refresh('old');
      expect(a, 'a');
      expect(r, 'r');
    });

    test('PeerLease.fromJson round-trip', () {
      final lease = PeerLease.fromJson({
        'peer_id': 'p1',
        'address': '10.20.0.2/24',
        'config': '[Interface]',
      });
      expect(lease.peerId, 'p1');
      expect(lease.address, '10.20.0.2/24');
    });
  });
}

class _FakeClient extends http.BaseClient {
  _FakeClient(this.handler);
  final Future<http.Response> Function(http.Request) handler;

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final res = await handler(request as http.Request);
    return http.StreamedResponse(
      Stream.value(res.bodyBytes),
      res.statusCode,
      headers: res.headers,
    );
  }
}
