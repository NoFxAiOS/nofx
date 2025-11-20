from flask import Flask, render_template, request, jsonify
import supabase_config

app = Flask(__name__)

# 初始化 Supabase 客户端
supabase = supabase_config.init_supabase()

@app.route('/')
def index():
    return render_template('index.html')

@app.route('/api/data')
def get_data():
    try:
        # 从 Supabase 获取数据示例
        response = supabase.table('your_table').select('*').execute()
        return jsonify(response.data)
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/api/data', methods=['POST'])
def add_data():
    try:
        data = request.json
        response = supabase.table('your_table').insert(data).execute()
        return jsonify(response.data)
    except Exception as e:
        return jsonify({'error': str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000, debug=True)
