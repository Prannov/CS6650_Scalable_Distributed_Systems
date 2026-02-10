from flask import Flask, request, jsonify
import boto3
import json
import re
from collections import Counter

app = Flask(__name__)
s3 = boto3.client('s3', region_name='us-east-1')

def parse_s3_url(s3_url):
    """Parse s3://bucket/key into (bucket, key)"""
    parts = s3_url.replace('s3://', '').split('/', 1)
    return parts[0], parts[1] if len(parts) > 1 else ''

@app.route('/')
def home():
    return jsonify({'status': 'Mapper service is running'})

@app.route('/map', methods=['GET'])
def map_words():
    try:
        chunk_url = request.args.get('url')
        print(f"Mapping {chunk_url}")
        
        # Parse and download chunk
        bucket, key = parse_s3_url(chunk_url)
        obj = s3.get_object(Bucket=bucket, Key=key)
        text = obj['Body'].read().decode('utf-8')
        
        print(f"Downloaded {len(text)} characters")
        
        # Count words (lowercase, remove punctuation)
        words = re.findall(r'\b[a-z]+\b', text.lower())
        word_counts = dict(Counter(words))
        
        print(f"Found {len(words)} words, {len(word_counts)} unique")
        
        # Save results to S3
        chunk_name = key.split('/')[-1].replace('.txt', '')
        output_key = f"mapped/{chunk_name}.json"
        s3.put_object(
            Bucket=bucket,
            Key=output_key,
            Body=json.dumps(word_counts).encode('utf-8')
        )
        
        output_url = f"s3://{bucket}/{output_key}"
        print(f"Saved results to {output_url}")
        
        return jsonify({
            'status': 'success',
            'output_url': output_url,
            'word_count': len(words),
            'unique_words': len(word_counts)
        })
    
    except Exception as e:
        print(f"Error: {str(e)}")
        import traceback
        traceback.print_exc()
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    print("Starting Mapper service on port 8080...")
    app.run(host='0.0.0.0', port=8080, debug=True)