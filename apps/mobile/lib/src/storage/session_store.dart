import 'dart:convert';

import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:shared_preferences/shared_preferences.dart';

import '../models/nms_models.dart';

class SessionStore {
  static const _secureStorage = FlutterSecureStorage();
  static const _sessionKey = 'nms_mobile_session';
  static const _lastHostKey = 'nms_mobile_last_host';
  static const _lastSiteKey = 'nms_mobile_last_site';

  Future<void> saveSession(NmsSession session) async {
    await _secureStorage.write(
      key: _sessionKey,
      value: jsonEncode(session.toJson()),
    );
    final prefs = await SharedPreferences.getInstance();
    await prefs.setString(_lastHostKey, session.baseUrl);
    await prefs.setString(_lastSiteKey, session.siteName);
  }

  Future<NmsSession?> loadSession() async {
    final raw = await _secureStorage.read(key: _sessionKey);
    if (raw == null || raw.isEmpty) {
      return null;
    }
    try {
      final decoded = jsonDecode(raw);
      if (decoded is! Map) {
        return null;
      }
      final session = NmsSession.fromJson(
        decoded.map((key, value) => MapEntry(key.toString(), value)),
      );
      if (session.token.isEmpty || session.expiresAt.isBefore(DateTime.now())) {
        await clear();
        return null;
      }
      return session;
    } on Object {
      await clear();
      return null;
    }
  }

  Future<String> loadLastHost() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_lastHostKey) ?? '';
  }

  Future<String> loadLastSiteName() async {
    final prefs = await SharedPreferences.getInstance();
    return prefs.getString(_lastSiteKey) ?? '';
  }

  Future<void> clear() async {
    await _secureStorage.delete(key: _sessionKey);
  }
}
