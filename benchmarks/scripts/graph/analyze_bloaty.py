#!/usr/bin/env python3

import argparse
import re
import sys


def parse_size(size_str):
    """Parse a size string like '333Ki', '2.47Mi', '808' into bytes."""
    size_str = size_str.strip()

    # Handle numeric-only values (bytes)
    if size_str.isdigit():
        return int(size_str)

    # Match number with optional unit
    match = re.match(r'([\d.]+)(Ki|Mi|Gi)?', size_str)
    if not match:
        return 0

    value = float(match.group(1))
    unit = match.group(2)

    if unit == 'Ki':
        return int(value * 1024)
    elif unit == 'Mi':
        return int(value * 1024 * 1024)
    elif unit == 'Gi':
        return int(value * 1024 * 1024 * 1024)
    else:
        return int(value)


def format_size(size_bytes):
    """Format bytes into a human-readable string."""
    if size_bytes < 1024:
        return f"{size_bytes} B"
    elif size_bytes < 1024 * 1024:
        return f"{size_bytes / 1024:.2f} KiB"
    elif size_bytes < 1024 * 1024 * 1024:
        return f"{size_bytes / (1024 * 1024):.2f} MiB"
    else:
        return f"{size_bytes / (1024 * 1024 * 1024):.2f} GiB"


def parse_bloaty_output(filepath):
    """Parse bloaty output file and return a list of (component_name, file_size_bytes, vm_size_bytes) tuples."""
    components = []

    with open(filepath, 'r') as f:
        for line in f:
            line = line.strip()

            # Skip headers, separators, and total line
            if not line or 'FILE SIZE' in line or '---' in line or 'TOTAL' in line:
                continue

            # Parse the line format:
            # percent  size  percent  size  component_name
            parts = line.split()
            if len(parts) < 5:
                continue

            # Extract file size (2nd column) and component name (last column onwards)
            try:
                file_size_str = parts[1]
                vm_size_str = parts[3]
                component_name = ' '.join(parts[4:])

                file_size_bytes = parse_size(file_size_str)
                vm_size_bytes = parse_size(vm_size_str)

                components.append((component_name, file_size_bytes, vm_size_bytes))
            except (ValueError, IndexError):
                continue

    return components


def main():
    parser = argparse.ArgumentParser(
        description='Analyze bloaty output and categorize components'
    )
    parser.add_argument(
        '--filepath',
        type=str,
        default='/tmp/out',
        help='Path to bloaty output file (default: /tmp/out)'
    )

    args = parser.parse_args()

    components = parse_bloaty_output(args.filepath)

    # Map categories to sets of components (fill in as needed)
    category_to_components = {
        # Add mappings here, e.g.:
        'init_rpc': {
            '/home/sigmaos/cpp/apps/cache/proto/get.pb.cc',
        },
        'rpc_stack': {
            '/home/sigmaos/cpp/rpc/clnt.cc',
            '/home/sigmaos/cpp/rpc/blob.cc',
            '/home/sigmaos/cpp/rpc/srv.cc',
            '/home/sigmaos/cpp/rpc/proto/rpc.pb.cc',
            '/home/sigmaos/cpp/rpc/spchannel/spchannel.cc',
            '/home/sigmaos/cpp/rpc/delegation/cache.cc',
            '/home/sigmaos/cpp/io/net/srv.cc',
            '/home/sigmaos/cpp/io/demux/srv.cc',
            '/home/sigmaos/cpp/io/frame/frame.cc',
            '/home/sigmaos/cpp/threadpool/threadpool.cc',
            '/home/sigmaos/cpp/io/conn/conn.cc',
            '/home/sigmaos/cpp/io/conn/tcp/tcp.cc',
            '/home/sigmaos/cpp/io/transport/transport.cc',
            '/home/sigmaos/cpp/io/demux/clnt.cc',
            '/home/sigmaos/cpp/io/transport/internal/callmap.cc',
            '/home/sigmaos/cpp/util/codec/codec.cc',
            '/home/sigmaos/cpp/shmem/segment.cc',
            '/home/sigmaos/cpp/shmem/shmem.cc',
        },
        'srv_proto': {
            '/home/sigmaos/cpp/apps/cossim/proto/cossim.pb.cc',
        },
        'app_impl': {
            '/home/sigmaos/cpp/apps/cossim/vec.cc',
            '/home/sigmaos/cpp/apps/cossim/srv.cc',
            '/home/sigmaos/cpp/user/cossim-srv-cpp/main.cc',
        },
        'log': {
            '/home/sigmaos/cpp/util/log/log.cc',
        },
        'perf': {
            '/home/sigmaos/cpp/util/perf/perf.cc',
            '/home/sigmaos/cpp/util/tracing/proto/tracing.pb.cc',
            '/home/sigmaos/cpp/util/metrics/server_metrics.cc',
        },
        'client_libs': {
            '/home/sigmaos/cpp/apps/cache/proto/cache.pb.cc',
            '/home/sigmaos/cpp/apps/cache/clnt.cc',
            '/home/sigmaos/cpp/proc/proc.pb.cc',
            '/home/sigmaos/cpp/proc/proc.cc',
            '/home/sigmaos/cpp/sigmap/sigmap.pb.cc',
            '/home/sigmaos/cpp/proxy/sigmap/sigmap.cc',
            '/home/sigmaos/cpp/apps/cache/shard.cc',
            '/home/sigmaos/cpp/serr/serr.cc',
            '/home/sigmaos/cpp/apps/cache/cache.cc',
            '/home/sigmaos/cpp/util/common/util.cc',
            '/home/sigmaos/cpp/proxy/sigmap/proto/spproxy.pb.cc',
            '/home/sigmaos/cpp/io/demux/internal/callmap.cc',
        },
        'exceptions': {
            '[section .gcc_except_table]',
        }
    }

    # Build reverse mapping: component -> category
    component_to_category = {}
    for category, component_set in category_to_components.items():
        for component in component_set:
            component_to_category[component] = category

    # Aggregate by category, separating defined vs undefined categories
    defined_categories = {}
    undefined_categories = {}

    for component_name, file_size, vm_size in components:
        # Look up category
        category = component_to_category.get(component_name, component_name)

        # Determine if this is a defined category or not
        is_defined = component_name in component_to_category

        if is_defined:
            if category not in defined_categories:
                defined_categories[category] = {'file_size': 0, 'vm_size': 0}
            defined_categories[category]['file_size'] += file_size
            defined_categories[category]['vm_size'] += vm_size
        else:
            if category not in undefined_categories:
                undefined_categories[category] = {'file_size': 0, 'vm_size': 0}
            undefined_categories[category]['file_size'] += file_size
            undefined_categories[category]['vm_size'] += vm_size

    # Print defined categories table
    print("DEFINED CATEGORIES")
    print("=" * 110)
    print(f"{'Category':<80} {'File Size':<15} {'VM Size':<15}")
    print("=" * 110)

    sorted_defined = sorted(defined_categories.items(), key=lambda x: x[1]['file_size'], reverse=True)

    total_defined_file_size = 0
    total_defined_vm_size = 0

    for category, sizes in sorted_defined:
        file_size_str = format_size(sizes['file_size'])
        vm_size_str = format_size(sizes['vm_size'])
        print(f"{category:<80} {file_size_str:<15} {vm_size_str:<15}")
        total_defined_file_size += sizes['file_size']
        total_defined_vm_size += sizes['vm_size']

    print("=" * 110)
    print(f"{'TOTAL':<80} {format_size(total_defined_file_size):<15} {format_size(total_defined_vm_size):<15}")
    print()

    # Print undefined categories table
    print("OTHER COMPONENTS")
    print("=" * 110)
    print(f"{'Component':<80} {'File Size':<15} {'VM Size':<15}")
    print("=" * 110)

    sorted_undefined = sorted(undefined_categories.items(), key=lambda x: x[1]['file_size'], reverse=True)

    total_undefined_file_size = 0
    total_undefined_vm_size = 0

    for category, sizes in sorted_undefined:
        file_size_str = format_size(sizes['file_size'])
        vm_size_str = format_size(sizes['vm_size'])
        print(f"{category:<80} {file_size_str:<15} {vm_size_str:<15}")
        total_undefined_file_size += sizes['file_size']
        total_undefined_vm_size += sizes['vm_size']

    print("=" * 110)
    print(f"{'TOTAL':<80} {format_size(total_undefined_file_size):<15} {format_size(total_undefined_vm_size):<15}")
    print()

    # Print overall total
    print("OVERALL TOTAL")
    print("=" * 110)
    total_file_size = total_defined_file_size + total_undefined_file_size
    total_vm_size = total_defined_vm_size + total_undefined_vm_size
    print(f"{'TOTAL':<80} {format_size(total_file_size):<15} {format_size(total_vm_size):<15}")


if __name__ == '__main__':
    main()
