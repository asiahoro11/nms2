import 'package:flutter/material.dart';

import '../api/nms_api_client.dart';
import '../models/nms_models.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({
    required this.session,
    required this.onLogout,
    super.key,
  });

  final NmsSession session;
  final Future<void> Function() onLogout;

  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  late Future<DashboardData> _dashboardFuture;

  @override
  void initState() {
    super.initState();
    _dashboardFuture = _loadDashboard();
  }

  Future<DashboardData> _loadDashboard() {
    return NmsApiClient(
      baseUrl: widget.session.baseUrl,
      token: widget.session.token,
    ).dashboard();
  }

  void _refresh() {
    setState(() {
      _dashboardFuture = _loadDashboard();
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.session.siteName),
        actions: [
          IconButton(
            tooltip: 'Refresh',
            onPressed: _refresh,
            icon: const Icon(Icons.refresh),
          ),
          IconButton(
            tooltip: 'Logout',
            onPressed: widget.onLogout,
            icon: const Icon(Icons.logout),
          ),
        ],
      ),
      body: SafeArea(
        child: FutureBuilder<DashboardData>(
          future: _dashboardFuture,
          builder: (context, snapshot) {
            if (snapshot.connectionState == ConnectionState.waiting) {
              return const Center(child: CircularProgressIndicator());
            }
            if (snapshot.hasError) {
              return _DashboardError(
                message: snapshot.error.toString(),
                onRetry: _refresh,
              );
            }
            final data = snapshot.requireData;
            return RefreshIndicator(
              onRefresh: () async => _refresh(),
              child: ListView(
                padding: const EdgeInsets.all(16),
                children: [
                  _Header(session: widget.session, data: data),
                  const SizedBox(height: 16),
                  _SummaryGrid(data: data),
                  const SizedBox(height: 16),
                  _SectionTitle(title: 'System'),
                  _MetricTile(
                    label: 'Uptime',
                    value: data.systemStats.uptime.isEmpty ? '-' : data.systemStats.uptime,
                  ),
                  _MetricTile(
                    label: 'Memory usage',
                    value: '${data.systemStats.memoryUsage.toStringAsFixed(1)}%',
                  ),
                  _MetricTile(
                    label: 'Runtime workers',
                    value: data.systemStats.goRoutines.toString(),
                  ),
                  const SizedBox(height: 16),
                  _SectionTitle(title: 'Recent events'),
                  if (data.recentEvents.isEmpty)
                    const _EmptyState(message: 'No recent events.')
                  else
                    ...data.recentEvents.take(8).map(_EventTile.new),
                ],
              ),
            );
          },
        ),
      ),
    );
  }
}

class _Header extends StatelessWidget {
  const _Header({required this.session, required this.data});

  final NmsSession session;
  final DashboardData data;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          data.systemName.isEmpty ? session.siteName : data.systemName,
          style: Theme.of(context).textTheme.headlineSmall,
        ),
        const SizedBox(height: 4),
        Text(
          '${session.baseUrl}  |  ${session.user.username} (${session.user.role})',
          style: Theme.of(context).textTheme.bodySmall,
        ),
        if (data.version.isNotEmpty) ...[
          const SizedBox(height: 4),
          Text('Version ${data.version}', style: Theme.of(context).textTheme.bodySmall),
        ],
      ],
    );
  }
}

class _SummaryGrid extends StatelessWidget {
  const _SummaryGrid({required this.data});

  final DashboardData data;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final columns = constraints.maxWidth > 560 ? 3 : 2;
        return GridView.count(
          crossAxisCount: columns,
          crossAxisSpacing: 12,
          mainAxisSpacing: 12,
          childAspectRatio: 1.55,
          shrinkWrap: true,
          physics: const NeverScrollableScrollPhysics(),
          children: [
            _SummaryCard(label: 'Devices', value: data.totalDevices.toString()),
            _SummaryCard(label: 'Online', value: data.onlineCount.toString()),
            _SummaryCard(label: 'Offline', value: data.offlineCount.toString()),
            _SummaryCard(label: 'Events', value: data.recentEvents.length.toString()),
          ],
        );
      },
    );
  }
}

class _SummaryCard extends StatelessWidget {
  const _SummaryCard({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return Card(
      elevation: 0,
      color: Theme.of(context).colorScheme.surfaceContainerHighest,
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            Text(label, style: Theme.of(context).textTheme.labelLarge),
            Text(value, style: Theme.of(context).textTheme.headlineMedium),
          ],
        ),
      ),
    );
  }
}

class _SectionTitle extends StatelessWidget {
  const _SectionTitle({required this.title});

  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Text(title, style: Theme.of(context).textTheme.titleMedium),
    );
  }
}

class _MetricTile extends StatelessWidget {
  const _MetricTile({required this.label, required this.value});

  final String label;
  final String value;

  @override
  Widget build(BuildContext context) {
    return ListTile(
      contentPadding: EdgeInsets.zero,
      title: Text(label),
      trailing: Text(value),
    );
  }
}

class _EventTile extends StatelessWidget {
  const _EventTile(this.event);

  final NmsEvent event;

  @override
  Widget build(BuildContext context) {
    return Card(
      elevation: 0,
      child: ListTile(
        title: Text(event.message.isEmpty ? event.eventType : event.message),
        subtitle: Text('${event.severity}  |  ${event.createdAt}'),
      ),
    );
  }
}

class _DashboardError extends StatelessWidget {
  const _DashboardError({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(message, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton(onPressed: onRetry, child: const Text('Retry')),
          ],
        ),
      ),
    );
  }
}

class _EmptyState extends StatelessWidget {
  const _EmptyState({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 24),
      child: Center(child: Text(message)),
    );
  }
}
