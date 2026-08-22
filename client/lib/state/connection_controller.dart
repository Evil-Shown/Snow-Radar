import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:wireguard_flutter/wireguard_flutter.dart';

import '../services/api_client.dart';
import '../services/keychain_service.dart';

enum TunnelStatus { disconnected, connecting, connected, failed }

class ConnectionState {
  const ConnectionState({
    this.status = TunnelStatus.disconnected,
    this.nodeId = 'sgp',
    this.stealthMode = false,
    this.address,
    this.error,
  });

  final TunnelStatus status;
  final String nodeId;
  final bool stealthMode;
  final String? address;
  final String? error;

  ConnectionState copyWith({
    TunnelStatus? status,
    String? nodeId,
    bool? stealthMode,
    String? address,
    String? error,
  }) =>
      ConnectionState(
        status: status ?? this.status,
        nodeId: nodeId ?? this.nodeId,
        stealthMode: stealthMode ?? this.stealthMode,
        address: address ?? this.address,
        error: error,
      );
}

class ConnectionController extends Notifier<ConnectionState> {
  late final WireGuardFlutterInterface _wg;
  ApiClient get _api => ref.read(apiClientProvider);

  @override
  ConnectionState build() {
    _wg = WireGuardFlutter.instance;
    return const ConnectionState();
  }

  Future<void> toggleStealth(bool on) async {
    if (state.status == TunnelStatus.connected) {
      await disconnect(); // protocol switch always forces a fresh tunnel
    }
    state = state.copyWith(stealthMode: on);
  }

  void selectNode(String nodeId) => state = state.copyWith(nodeId: nodeId);

  Future<void> connect(String accessToken) async {
    try {
      state = state.copyWith(status: TunnelStatus.connecting, error: null);

      final keychain = ref.read(keychainProvider);
      final publicKey = await keychain.ensureKeyPair();

      final lease = await _api.connect(
        accessToken: accessToken,
        nodeId: state.nodeId,
        publicKey: publicKey,
        stealth: state.stealthMode,
      );

      // Merge device-held private key into the server-rendered config.
      final priv = await keychain.privateKey();
      if (priv == null) throw Exception('local key material missing');
      final fullConfig = _mergePrivateKey(lease.config, priv);

      await _wg.setConfig(
        wgQuickConfig: fullConfig,
        token: '',
        title: 'snowradar-${lease.peerId}',
      );
      await _wg.connect();

      state = state.copyWith(status: TunnelStatus.connected, address: lease.address);
    } on PaywallRequired {
      state = state.copyWith(status: TunnelStatus.failed, error: 'subscription required');
    } catch (e) {
      state = state.copyWith(status: TunnelStatus.failed, error: e.toString());
    }
  }

  Future<void> disconnect() async {
    try {
      await _wg.disconnect();
    } finally {
      state = state.copyWith(status: TunnelStatus.disconnected, address: null);
    }
  }

  /// The server never sees the private key; only the client can assemble a
  /// complete [Interface] block. Kept string-safe and idempotent.
  String _mergePrivateKey(String serverConfig, String privB64) {
    final sb = StringBuffer('[Interface]\n');
    var inserted = false;
    for (final line in const LineSplitter().convert(serverConfig)) {
      sb.writeln(line);
      if (!inserted && line.startsWith('Address')) {
        sb.writeln('PrivateKey = $privB64');
        inserted = true;
      }
    }
    return sb.toString();
  }
}

final apiClientProvider = Provider<ApiClient>((ref) {
  // Release builds MUST pin the API certificate (audit #15). The pin is
  // injected at build time; dev builds may run unpinned.
  const pin = String.fromEnvironment('API_CERT_SHA256');
  return ApiClient(
    baseUrl: const String.fromEnvironment('API_BASE_URL', defaultValue: 'http://127.0.0.1:8080'),
    pinnedSha256: pin.isEmpty ? null : pin,
  );
});

final keychainProvider = Provider<KeychainService>((ref) => KeychainService());

final connectionProvider =
    NotifierProvider<ConnectionController, ConnectionState>(ConnectionController.new);
