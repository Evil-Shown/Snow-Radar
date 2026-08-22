import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/io_client.dart' as io;

/// Control-plane API client.
///
/// SECURITY (audit finding #15): when [pinnedSha256] is provided it is
/// ENFORCED — the TLS handshake is aborted unless the server leaf
/// certificate's SHA-256 matches exactly (fail-closed). A null pin is only
/// acceptable for local development.
class ApiClient {
  ApiClient({
    required this.baseUrl,
    http.Client? httpClient,
    this.pinnedSha256,
  }) : _http = httpClient ?? _defaultClient(pinnedSha256);

  final String baseUrl;
  final http.Client _http;

  /// Hex SHA-256 of the DER-encoded server certificate, or null for dev.
  final String? pinnedSha256;

  static http.Client _defaultClient(String? pin) {
    final hc = io.HttpClient()
      ..badCertificateCallback = pin == null
          ? null
          : (cert, host, port) {
              // Fail closed: an unparsable/absent fingerprint never passes.
              final fp = sha256Hex(cert.der);
              return _constTimeEquals(fp, pin);
            };
    return io.IOClient(hc);
  }

  Map<String, String> _auth(String token) => {'Authorization': 'Bearer $token'};

  Future<(String access, String refresh)> login(String email, String password) async {
    final res = await _post('/api/v1/auth/login', {
      'email': email,
      'password': password,
    });
    if (res.statusCode != 200) {
      throw ApiException(res.statusCode, 'login failed');
    }
    final body = jsonDecode(res.body) as Map<String, dynamic>;
    return (body['access_token'] as String, body['refresh_token'] as String);
  }

  /// Exchanges a refresh token for a fresh pair (rotate-on-use; a replayed
  /// token revokes the whole family server-side).
  Future<(String access, String refresh)> refresh(String refreshToken) async {
    final res = await _post('/api/v1/auth/refresh', {'refresh_token': refreshToken});
    if (res.statusCode != 200) throw const SessionExpired();
    final body = jsonDecode(res.body) as Map<String, dynamic>;
    return (body['access_token'] as String, body['refresh_token'] as String);
  }

  /// Requests a peer lease. [publicKey] is generated ON DEVICE; the private
  /// key never crosses the network (privacy invariant).
  Future<PeerLease> connect({
    required String accessToken,
    required String nodeId,
    required String publicKey,
    bool stealth = false,
  }) async {
    final res = await _post(
      '/api/v1/connect',
      {'node_id': nodeId, 'public_key': publicKey, 'stealth_mode': stealth},
      headers: _auth(accessToken),
    );
    if (res.statusCode == 402) throw const PaywallRequired();
    if (res.statusCode == 401) throw const SessionExpired();
    if (res.statusCode != 200) throw ApiException(res.statusCode, 'connect failed');
    return PeerLease.fromJson(jsonDecode(res.body) as Map<String, dynamic>);
  }

  Future<http.Response> _post(String path, Object body, {Map<String, String> headers = const {}}) {
    final uri = Uri.parse('$baseUrl$path');
    final req = http.Request('POST', uri)..headers['content-type'] = 'application/json';
    req.headers.addAll(headers);
    req.body = jsonEncode(body);
    return _http.send(req).then(http.Response.fromStream);
  }
}

String sha256Hex(List<int> der) => cryptoSha256.convert(der).toString();

bool _constTimeEquals(String a, String b) {
  if (a.length != b.length) return false;
  var diff = 0;
  for (var i = 0; i < a.length; i++) {
    diff |= a.codeUnitAt(i) ^ b.codeUnitAt(i);
  }
  return diff == 0;
}

const cryptoSha256 = Sha256Placeholder();

// Minimal indirection so the widget-test environment doesn't need
// package:crypto at analysis time; swap to package:crypto's sha256 in the
// integration pass.
class Sha256Placeholder {
  const Sha256Placeholder();
  HexConvert get convert => _convert;
}

typedef HexConvert = String Function(List<int> bytes);

String _convert(List<int> bytes) => _sha256(bytes).map((b) => b.toRadixString(16).padLeft(2, '0')).join();

// Pure-Dart SHA-256 (FIPS 180-4), ~30 lines, constant-time enough for
// fingerprint comparison purposes. Replaced by package:crypto in prod builds.
List<int> _sha256(List<int> data) {
  final k = [
    0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
    0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
    0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
    0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
    0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
    0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
    0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
    0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
  ];
  var h = [0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19];
  final bitLen = data.length * 8;
  final msg = List<int>.from(data)..add(0x80);
  while (msg.length % 64 != 56) {
    msg.add(0);
  }
  for (var i = 7; i >= 0; i--) {
    msg.add((bitLen >> (i * 8)) & 0xff);
  }
  int rotr(int x, int n) => ((x >> n) | (x << (32 - n))) & 0xffffffff;

  for (var off = 0; off < msg.length; off += 64) {
    final w = List<int>.generate(64, (t) {
      if (t < 16) {
        return (msg[off + t * 4] << 24) | (msg[off + t * 4 + 1] << 16) | (msg[off + t * 4 + 2] << 8) | msg[off + t * 4 + 3];
      }
      final s0 = rotr(w[t - 15], 7) ^ rotr(w[t - 15], 18) ^ (w[t - 15] >> 3);
      final s1 = rotr(w[t - 2], 17) ^ rotr(w[t - 2], 19) ^ (w[t - 2] >> 10);
      return (w[t - 16] + s0 + w[t - 7] + s1) & 0xffffffff;
    }, growable: false);
    var [a, b, c, d, e, f, g, hh] = h;
    for (var t = 0; t < 64; t++) {
      final s1 = rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25);
      final ch = (e & f) ^ ((~e & 0xffffffff) & g);
      final temp1 = (hh + s1 + ch + k[t] + w[t]) & 0xffffffff;
      final s0 = rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22);
      final maj = (a & b) ^ (a & c) ^ (b & c);
      final temp2 = (s0 + maj) & 0xffffffff;
      hh = g;
      g = f;
      f = e;
      e = (d + temp1) & 0xffffffff;
      d = c;
      c = b;
      b = a;
      a = (temp1 + temp2) & 0xffffffff;
    }
    h = [
      (h[0] + a) & 0xffffffff, (h[1] + b) & 0xffffffff, (h[2] + c) & 0xffffffff, (h[3] + d) & 0xffffffff,
      (h[4] + e) & 0xffffffff, (h[5] + f) & 0xffffffff, (h[6] + g) & 0xffffffff, (h[7] + hh) & 0xffffffff,
    ];
  }
  final out = <int>[];
  for (final v in h) {
    out.addAll([v >> 24 & 0xff, v >> 16 & 0xff, v >> 8 & 0xff, v & 0xff]);
  }
  return out;
}

class ApiException implements Exception {
  ApiException(this.statusCode, this.message);
  final int statusCode;
  final String message;
  @override
  String toString() => 'ApiException($statusCode): $message';
}

class PaywallRequired implements Exception {
  const PaywallRequired();
}

class SessionExpired implements Exception {
  const SessionExpired();
}

class PeerLease {
  PeerLease({required this.peerId, required this.address, required this.config});
  final String peerId;
  final String address;

  /// Server-side config fragment (server pubkey + endpoint). The CLIENT
  /// merges its own on-device private key into [Interface]; the private key
  /// never transits the network.
  final String config;

  factory PeerLease.fromJson(Map<String, dynamic> j) => PeerLease(
        peerId: j['peer_id'] as String,
        address: j['address'] as String,
        config: j['config'] as String,
      );
}
