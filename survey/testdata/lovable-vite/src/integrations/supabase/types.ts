export type Json = string | number | boolean | null | { [key: string]: Json | undefined } | Json[]

export type Database = {
  graphql_public: {
    Tables: {
      [_ in never]: never
    }
    Views: {
      [_ in never]: never
    }
  }
  public: {
    Tables: {
      jobs: {
        Row: {
          address: string
          created_at: string
          customer: string
          id: string
          quote_cents: number | null
          status: string
          user_id: string
        }
        Insert: {
          address: string
          customer: string
          id?: string
          user_id: string
        }
        Update: {
          status?: string
        }
        Relationships: []
      }
      profiles: {
        Row: {
          created_at: string
          full_name: string | null
          id: string
        }
        Insert: {
          id: string
        }
        Update: {
          full_name?: string | null
        }
        Relationships: []
      }
    }
    Views: {
      [_ in never]: never
    }
    Functions: {
      [_ in never]: never
    }
  }
}
