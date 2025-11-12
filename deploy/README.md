# teletrack: build and deployment

1. Rename `inventory.example.yml` to `inventory.yml`. Fill with your values.
2. Put your `config.prod.json`.
2. Run: `ansible-playbook -i inventory.yml playbook.yml`