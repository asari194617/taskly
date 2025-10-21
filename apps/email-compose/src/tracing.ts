import tracer from 'dd-trace';

// Initialize Datadog APM tracer if enabled
if (process.env.DD_TRACE_ENABLED !== 'false') {
  tracer.init({
    logInjection: true,
    runtimeMetrics: false as any,
    service: process.env.DD_SERVICE || 'email-compose',
    env: process.env.DD_ENV || process.env.NODE_ENV || 'development',
    version: process.env.DD_VERSION,
  } as any);
  tracer.use('http');
}

export default tracer;
