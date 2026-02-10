from flask import Flask, request, jsonify
import boto3
import math

app = Flask(__name__)
s3 = boto3.client('s3', region_name='us-east-1')

def parse_s3_url(s3_url):
    """Parse s3://bucket/key into (bucket, key)"""
    parts = s3_url.replace('s3://', '').split('/', 1)
    return parts[0], parts[1] if len(parts) > 1 else ''

@app.route('/')
def home():
    return jsonify({'status': 'Splitter service is running'})

@app.route('/split', methods=['GET'])
def split_file():
    try:
        source_url = request.args.get('url')
        num_chunks = int(request.args.get('chunks', 3))
        
        print(f"Splitting {source_url} into {num_chunks} chunks")
        
        # Parse S3 URL
        bucket, key = parse_s3_url(source_url)
        
        # Download file
        obj = s3.get_object(Bucket=bucket, Key=key)
        content = obj['Body'].read().decode('utf-8')
        
        print(f"Downloaded {len(content)} characters from {source_url}")
        
        # Split into chunks
        chunk_size = math.ceil(len(content) / num_chunks)
        chunk_urls = []
        
        for i in range(num_chunks):
            start = i * chunk_size
            end = min((i + 1) * chunk_size, len(content))
            chunk = content[start:end]
            
            chunk_key = f"chunks/chunk_{i}.txt"
            s3.put_object(Bucket=bucket, Key=chunk_key, Body=chunk.encode('utf-8'))
            chunk_urls.append(f"s3://{bucket}/{chunk_key}")
            print(f"Created chunk {i}: {len(chunk)} characters")
        
        return jsonify({
            'status': 'success',
            'chunk_urls': chunk_urls,
            'num_chunks': len(chunk_urls)
        })
    
    except Exception as e:
        print(f"Error: {str(e)}")
        import traceback
        traceback.print_exc()
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    print("Starting Splitter service on port 8080...")
    app.run(host='0.0.0.0', port=8080, debug=True)