from flask import Flask, request, jsonify
import boto3
import json
from collections import defaultdict

app = Flask(__name__)
s3 = boto3.client('s3', region_name='us-east-1')

def parse_s3_url(s3_url):
    """Parse s3://bucket/key into (bucket, key)"""
    parts = s3_url.replace('s3://', '').split('/', 1)
    return parts[0], parts[1] if len(parts) > 1 else ''

@app.route('/')
def home():
    return jsonify({'status': 'Reducer service is running'})

@app.route('/reduce', methods=['POST'])
def reduce_results():
    try:
        data = request.get_json()
        mapper_urls = data.get('urls', [])
        
        print(f"Reducing {len(mapper_urls)} mapper results")
        
        # Aggregate all mapper results
        final_counts = defaultdict(int)
        bucket = None
        
        for url in mapper_urls:
            print(f"Processing {url}")
            bucket, key = parse_s3_url(url)
            obj = s3.get_object(Bucket=bucket, Key=key)
            mapper_counts = json.loads(obj['Body'].read().decode('utf-8'))
            
            for word, count in mapper_counts.items():
                final_counts[word] += count
        
        print(f"Aggregated {len(final_counts)} unique words")
        
        # Sort by frequency
        sorted_counts = dict(sorted(
            final_counts.items(),
            key=lambda x: x[1],
            reverse=True
        ))
        
        # Save final results
        output_key = "final/word_counts.json"
        s3.put_object(
            Bucket=bucket,
            Key=output_key,
            Body=json.dumps(sorted_counts, indent=2).encode('utf-8')
        )
        
        output_url = f"s3://{bucket}/{output_key}"
        top_10 = list(sorted_counts.items())[:10]
        
        print(f"Saved final results to {output_url}")
        print(f"Top 10 words: {top_10}")
        
        return jsonify({
            'status': 'success',
            'output_url': output_url,
            'total_unique_words': len(sorted_counts),
            'top_10': top_10
        })
    
    except Exception as e:
        print(f"Error: {str(e)}")
        import traceback
        traceback.print_exc()
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    print("Starting Reducer service on port 8080...")
    app.run(host='0.0.0.0', port=8080, debug=True)