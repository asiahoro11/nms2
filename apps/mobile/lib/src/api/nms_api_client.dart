import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;

import '../models/nms_models.dart';

class NmsApiClient {
  NmsApiClient({
    required String baseUrl,
    this.token,
    http.Client? httpClient,
  })  : baseUrl = normalizeBaseUrl(baseUrl),
        _httpClient = httpClient ?? http.Client();

  final String baseUrl;
  final String? token;
  final http.Client _httpClient;

  static String normalizeBaseUrl(String value) {
    final trimmed = value.trim();
    if (trimmed.isEmpty) {
      return trimmed;
    }
    final withScheme = trimmed.startsWith('http://') || trimmed.startsWith('https://')
        ? trimmed
        : 'https://$trimmed';
    return withScheme.replaceFirst(RegExp(r'/+$'), '');
  }

  Future<Map<String, dynamic>> getSystemInfo() {
    return _get('/api/v1/system/info', authenticated: false);
  }

  Future<LoginResult> login({
    required String username,
    required String password,
  }) async {
    final payload = await _post(
      '/api/v1/auth/login',
      authenticated: false,
      body: {'username': username, 'password': password},
    );
    return LoginResult.fromJson(_requireMap(payload['data']));
  }

  Future<LoginResult> verifyTwoFactor({
    required String code,
    required String challengeToken,
    String? username,
    String? method,
  }) async {
    final payload = await _post(
      '/api/v1/auth/verify-2fa',
      authenticated: false,
      body: {
        'code': code,
        'challenge_token': challengeToken,
        if (username != null) 'username': username,
        if (method != null) 'method': method,
      },
    );
    return LoginResult.fromJson(_requireMap(payload['data']));
  }

  Future<UserProfile> currentUser() async {
    final payload = await _get('/api/v1/auth/me');
    return UserProfile.fromJson(_requireMap(payload['data']));
  }

  Future<DashboardData> dashboard() async {
    final payload = await _get('/api/v1/dashboard');
    return DashboardData.fromJson(_requireMap(payload['data']));
  }

  Future<void> logout() async {
    try {
      await _post('/api/v1/auth/logout', body: const {});
    } on Object {
      // Local logout must still clear the device session.
    }
  }

  Future<Map<String, dynamic>> _get(
    String path, {
    bool authenticated = true,
  }) async {
    final response = await _send(
      () => _httpClient.get(_uri(path), headers: _headers(authenticated)),
    );
    return _decodeEnvelope(response);
  }

  Future<Map<String, dynamic>> _post(
    String path, {
    required Map<String, dynamic> body,
    bool authenticated = true,
  }) async {
    final response = await _send(
      () => _httpClient.post(
        _uri(path),
        headers: _headers(authenticated),
        body: jsonEncode(body),
      ),
    );
    return _decodeEnvelope(response);
  }

  Future<http.Response> _send(Future<http.Response> Function() request) async {
    try {
      return await request().timeout(const Duration(seconds: 12));
    } on SocketException {
      throw const NmsApiException('Cannot reach the NMS host.');
    } on TimeoutException {
      throw const NmsApiException('Connection timed out.');
    } on FormatException {
      throw const NmsApiException('NMS returned an invalid response.');
    }
  }

  Uri _uri(String path) => Uri.parse('$baseUrl$path');

  Map<String, String> _headers(bool authenticated) {
    return {
      'Accept': 'application/json',
      'Content-Type': 'application/json',
      if (authenticated && token != null) 'Authorization': 'Bearer $token',
    };
  }

  Map<String, dynamic> _decodeEnvelope(http.Response response) {
    final dynamic decoded = response.body.isEmpty ? <String, dynamic>{} : jsonDecode(response.body);
    final map = _requireMap(decoded);
    final success = map['success'] == true;
    if (response.statusCode >= 400 || !success) {
      final message = map['error']?.toString() ??
          map['message']?.toString() ??
          'Request failed (${response.statusCode}).';
      throw NmsApiException(message, statusCode: response.statusCode);
    }
    return map;
  }

  Map<String, dynamic> _requireMap(Object? value) {
    if (value is Map<String, dynamic>) {
      return value;
    }
    if (value is Map) {
      return value.map((key, val) => MapEntry(key.toString(), val));
    }
    throw const NmsApiException('NMS returned an unexpected response.');
  }
}

class NmsApiException implements Exception {
  const NmsApiException(this.message, {this.statusCode});

  final String message;
  final int? statusCode;

  @override
  String toString() => message;
}
