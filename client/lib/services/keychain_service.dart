import 'dart:convert';
import 'dart:math';
import 'dart:typed_data';

import 'package:cryptography/cryptography.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// On-device WireGuard key material.
///
/// PRIVACY INVARIANT (SECURITY.md): the private key is generated here and
/// stored ONLY in platform secure storage (Android Keystore / iOS Keychain).
/// Only the PUBLIC key is ever sent to the control plane.
///
/// AUDIT NOTE (#16): an earlier draft implemented X25519 by hand. Deleted.
/// Crypto comes from package:cryptography (audited, constant-time backends),
/// never from this repo.
class KeychainService {
  KeychainService({FlutterSecureStorage? storage})
      : _storage = storage ??
            const FlutterSecureStorage(
              aOptions: AndroidOptions(encryptedSharedPreferences: true),
              iOptions: IOSOptions(accessibility: KeychainAccessibility.when_unlocked),
            );

  final FlutterSecureStorage _storage;
  static const _privKey = 'wg_private_key';
  static const _pubKey = 'wg_public_key';

  static final _algorithm = X25519();

  /// Returns the device's public key, generating the pair on first call.
  Future<String> ensureKeyPair() async {
    final existing = await _storage.read(key: _pubKey);
    if (existing != null) return existing;

    final keyPair = await _algorithm.newKeyPair();
    final privBytes = Uint8List.fromList(await keyPair.extractPrivateKeyBytes());
    final pubBytes = Uint8List.fromList(
      await keyPair.extractPublicKey().then((pk) => pk.bytes),
    );

    // Clamp per WireGuard spec before persisting.
    privBytes[0] &= 248;
    privBytes[31] &= 127;
    privBytes[31] |= 64;

    await _storage.write(key: _privKey, value: base64.encode(privBytes));
    await _storage.write(key: _pubKey, value: base64.encode(pubBytes));
    return base64.encode(pubBytes);
  }

  Future<String?> privateKey() => _storage.read(key: _privKey);

  Future<void> destroy() async {
    await _storage.delete(key: _privKey);
    await _storage.delete(key: _pubKey);
  }
}

Uint8List secureRandom32() {
  final rng = Random.secure();
  return Uint8List.fromList(List.generate(32, (_) => rng.nextInt(256)));
}
