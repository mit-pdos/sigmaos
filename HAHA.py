import pandas as pd
import numpy as np
import gc
import os
from typing import Optional, Iterator

def create_df1(size=500000):
    # Create first dataframe with randomly distributed keys
    # Using multiple key columns to force more complex join conditions
    return pd.DataFrame({
        'key1': np.random.randint(1, 1000, size=size),
        'key2': np.random.randint(1, 100, size=size),
        'text_data': np.random.choice(['A', 'B', 'C', 'D', 'E'] * 20, size=size),
        'value1': np.random.randn(size),
        'value2': np.random.randn(size),
        'value3': [np.random.bytes(10) for _ in range(size)],  # Unique bytes per row
        'timestamp': pd.date_range(start='2020-01-01', periods=size, freq='s')
    })

def create_df2(size=400000):
    # Create second dataframe with similar key distribution
    # but different enough to cause complex matching
    return pd.DataFrame({
        'key1': np.random.randint(1, 1000, size=size),
        'key2': np.random.randint(1, 100, size=size),
        'metric1': np.random.randn(size),
        'metric2': np.random.randn(size),
        'metric3': [np.random.bytes(10) for _ in range(size)],  # Unique bytes per row
        'category': np.random.choice(['X', 'Y', 'Z'], size=size),
        'date': pd.date_range(start='2020-01-01', periods=size, freq='min')
    })
def get_size_info(df, name):
    memory_usage = df.memory_usage(deep=True).sum() / (1024**3)  # Convert to GB
    return f"{name} Shape: {df.shape}, Memory: {memory_usage:.2f} GB"

# print("Creating dataframes...")
#df1 = create_df1()
#df2 = create_df2()
# print(get_size_info(df1, "DataFrame 1"))
# print(get_size_info(df2, "DataFrame 2"))
# df1.to_csv('A.csv', index=False)
# df2.to_csv('B.csv', index=False)
# del df1
# del df2

import resource
import psutil

def limit_memory_relative(additional_gb=0.1):
    """
    Limit the memory usage to current usage plus specified additional amount.
    Args:
        additional_gb (float): Additional memory allowance in gigabytes
    """
    # Get current memory usage
    current_process = psutil.Process()
    current_memory_bytes = current_process.memory_info().rss
    current_memory_gb = current_memory_bytes / 1024 / 1024 / 1024

    # Calculate new limit
    total_limit_gb = current_memory_gb + additional_gb
    total_limit_bytes = int(total_limit_gb * 1024 * 1024 * 1024)

    # Set the soft and hard limits
    resource.setrlimit(resource.RLIMIT_AS, (total_limit_bytes, total_limit_bytes))

    print(f"Current memory usage: {current_memory_gb:.2f} GB")
    print(f"Additional allowance: {additional_gb:.2f} GB")
    print(f"Total memory limit set to: {total_limit_gb:.2f} GB")

# Usage
limit_memory_relative(0.41)  # Allow 0.1 GB more than current usage
# Run garbage collection to free up memory
gc.collect()
def load_csv_chunked(
    file_path: str,
    chunk_size: int = 100000,
    columns: Optional[list] = None,
    **csv_kwargs
) -> Iterator[pd.DataFrame]:
    """
    Load a CSV file in chunks to manage memory usage.

    Parameters:
    -----------
    file_path : str
        Path to the CSV file
    chunk_size : int, default 100000
        Number of rows to load in each chunk
    columns : list, optional
        Specific columns to load. If None, loads all columns.
    csv_kwargs : dict
        Additional arguments to pass to pd.read_csv

    Yields:
    -------
    pd.DataFrame
        DataFrame chunk with specified number of rows
    """
    # Create CSV reader iterator
    csv_iter = pd.read_csv(
        file_path,
     #   usecols=columns,
        chunksize=chunk_size,
        **csv_kwargs
    )
    
    i = 0
    # Yield chunks
    for chunk in csv_iter:
     # if (i<2000):
        yield chunk
        i+=1

def process_csv_file(
    file_path: str,
    chunk_size: int = 100000,
    columns: Optional[list] = None,
    **csv_kwargs
) -> pd.DataFrame:
    """
    Process a CSV file in chunks and combine results.
    Use this when you need the entire DataFrame but want to process it in chunks.

    Parameters:
    -----------
    file_path : str
        Path to the CSV file
    chunk_size : int, default 100000
        Number of rows to load in each chunk
    columns : list, optional
        Specific columns to load. If None, loads all columns.
    csv_kwargs : dict
        Additional arguments to pass to pd.read_csv

    Returns:
    --------
    pd.DataFrame
        Combined DataFrame from all chunks
    """
    chunks = []
    total_rows = 0

    # Get total file size for progress monitoring
    file_size = os.path.getsize(file_path)

    for i, chunk in enumerate(load_csv_chunked(file_path, chunk_size, columns, **csv_kwargs)):
        chunks.append(chunk)
        total_rows += len(chunk)
        # Print progress
        if (i%500==0):
            print(f"Processed chunk {i+1}: {total_rows:,} rows", end='\r')

    print(f"\nCompleted loading {total_rows:,} total rows")
    return pd.concat(chunks, ignore_index=True)
df1 = process_csv_file('A.csv', chunk_size=500)
current_process = psutil.Process()
current_memory_bytes = current_process.memory_info().rss
current_memory_gb = current_memory_bytes / 1024 / 1024 / 1024
print(f"Current memory usage: {current_memory_gb:.2f} GB")
df2 = process_csv_file('A.csv', chunk_size=500)
#The pandas join operation
# try:
#     print("\nAttempting join operation...")
#     # Using merge with multiple conditions and sorted data
#     # This will cause high memory usage during the operation
#     result = pd.merge(
#         df1.sort_values(['key1', 'key2']),  # Sorting operation requires memory
#         df2.sort_values(['key1', 'key2']),  # Sorting operation requires memory
#         on=['key1', 'key2'],
#         how='inner'
#     )

#     print("\nJoin completed successfully!")
#     print(get_size_info(result, "Result DataFrame"))

#     # Calculate some statistics to show the data is valid
#     print("\nResult Statistics:")
#     print(f"Number of unique key1 values: {result['key1'].nunique()}")
#     print(f"Number of unique key2 values: {result['key2'].nunique()}")

# except Exception as e:
#     print("\nError during join operation:", str(e))