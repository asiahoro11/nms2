class NmsSession {
  const NmsSession({
    required this.siteName,
    required this.baseUrl,
    required this.token,
    required this.expiresAt,
    required this.user,
  });

  final String siteName;
  final String baseUrl;
  final String token;
  final DateTime expiresAt;
  final UserProfile user;

  Map<String, dynamic> toJson() {
    return {
      'site_name': siteName,
      'base_url': baseUrl,
      'token': token,
      'expires_at': expiresAt.millisecondsSinceEpoch,
      'user': user.toJson(),
    };
  }

  static NmsSession fromJson(Map<String, dynamic> json) {
    return NmsSession(
      siteName: json['site_name']?.toString() ?? 'NMS Site',
      baseUrl: json['base_url']?.toString() ?? '',
      token: json['token']?.toString() ?? '',
      expiresAt: DateTime.fromMillisecondsSinceEpoch(_asInt(json['expires_at'])),
      user: UserProfile.fromJson(_asMap(json['user'])),
    );
  }
}

class LoginResult {
  const LoginResult({
    required this.token,
    required this.user,
    required this.expiresAt,
    required this.requiresTwoFactor,
    required this.requirePasswordChange,
    this.twoFactorMethod,
    this.challengeToken,
  });

  final String token;
  final UserProfile user;
  final DateTime expiresAt;
  final bool requiresTwoFactor;
  final bool requirePasswordChange;
  final String? twoFactorMethod;
  final String? challengeToken;

  bool get hasToken => token.isNotEmpty;

  factory LoginResult.fromJson(Map<String, dynamic> json) {
    final expiresSeconds = _asInt(json['expires_at']);
    return LoginResult(
      token: json['token']?.toString() ?? '',
      user: UserProfile.fromJson(_asMap(json['user'])),
      expiresAt: DateTime.fromMillisecondsSinceEpoch(expiresSeconds * 1000),
      requiresTwoFactor: json['requires_two_factor'] == true,
      requirePasswordChange: json['require_password_change'] == true,
      twoFactorMethod: json['two_factor_method']?.toString(),
      challengeToken: json['challenge_token']?.toString(),
    );
  }
}

class UserProfile {
  const UserProfile({
    required this.id,
    required this.username,
    required this.role,
    required this.isActive,
  });

  final int id;
  final String username;
  final String role;
  final bool isActive;

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'username': username,
      'role': role,
      'is_active': isActive,
    };
  }

  factory UserProfile.fromJson(Map<String, dynamic> json) {
    return UserProfile(
      id: _asInt(json['id']),
      username: json['username']?.toString() ?? '',
      role: json['role']?.toString() ?? '',
      isActive: json['is_active'] != false,
    );
  }
}

class DashboardData {
  const DashboardData({
    required this.version,
    required this.systemName,
    required this.totalDevices,
    required this.onlineCount,
    required this.offlineCount,
    required this.systemStats,
    required this.recentEvents,
  });

  final String version;
  final String systemName;
  final int totalDevices;
  final int onlineCount;
  final int offlineCount;
  final SystemStats systemStats;
  final List<NmsEvent> recentEvents;

  factory DashboardData.fromJson(Map<String, dynamic> json) {
    return DashboardData(
      version: json['version']?.toString() ?? '',
      systemName: json['system_name']?.toString() ?? 'NMS',
      totalDevices: _asInt(json['total_devices']),
      onlineCount: _asInt(json['online_count']),
      offlineCount: _asInt(json['offline_count']),
      systemStats: SystemStats.fromJson(_asMap(json['system_stats'])),
      recentEvents: _asList(json['recent_events'])
          .map((item) => NmsEvent.fromJson(_asMap(item)))
          .toList(),
    );
  }
}

class SystemStats {
  const SystemStats({
    required this.uptime,
    required this.memoryUsage,
    required this.goRoutines,
  });

  final String uptime;
  final double memoryUsage;
  final int goRoutines;

  factory SystemStats.fromJson(Map<String, dynamic> json) {
    return SystemStats(
      uptime: json['uptime']?.toString() ?? '',
      memoryUsage: _asDouble(json['memory_usage']),
      goRoutines: _asInt(json['go_routines']),
    );
  }
}

class NmsEvent {
  const NmsEvent({
    required this.id,
    required this.eventType,
    required this.severity,
    required this.message,
    required this.createdAt,
  });

  final int id;
  final String eventType;
  final String severity;
  final String message;
  final String createdAt;

  factory NmsEvent.fromJson(Map<String, dynamic> json) {
    return NmsEvent(
      id: _asInt(json['id']),
      eventType: json['event_type']?.toString() ?? '',
      severity: json['severity']?.toString() ?? '',
      message: json['message']?.toString() ?? '',
      createdAt: json['created_at']?.toString() ?? '',
    );
  }
}

int _asInt(Object? value) {
  if (value is int) {
    return value;
  }
  if (value is num) {
    return value.toInt();
  }
  return int.tryParse(value?.toString() ?? '') ?? 0;
}

double _asDouble(Object? value) {
  if (value is double) {
    return value;
  }
  if (value is num) {
    return value.toDouble();
  }
  return double.tryParse(value?.toString() ?? '') ?? 0;
}

Map<String, dynamic> _asMap(Object? value) {
  if (value is Map<String, dynamic>) {
    return value;
  }
  if (value is Map) {
    return value.map((key, val) => MapEntry(key.toString(), val));
  }
  return <String, dynamic>{};
}

List<Object?> _asList(Object? value) {
  if (value is List) {
    return value;
  }
  return const [];
}
